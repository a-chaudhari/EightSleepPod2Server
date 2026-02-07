import struct
import argparse
import zlib
from Crypto.PublicKey import RSA

"""
Big Endian Byte Order
0: magic value: 6s
1: header crc: 4s
2: unknown: 2s
3: firmware size w/o header: I
4: fw crc w/o header: 4s
5: fw timestamp: I
6: unknown: 5s
7: fw version: 4s
8: unknown: 2s
9: build commit: 4s
10: unknown and padding: 473s
"""

HEADER_STRUCT_FORMAT = '>6s 4s 2s I 4s I 5s 4s 2s 4s 473s'
HEADER_START_OFFSET = 0x2_0000
HEADER_LEN = 512
FW_START_OFFSET = 0x2_0200


class FirmwareUpdater:
    firmware_data : bytearray

    def __init__(self, firmware_path: str):
        self.firmware_path = firmware_path
        self.open_firmware(firmware_path)

    def write_firmware(self):
        with open(self.firmware_path, 'wb') as f:
            f.write(self.firmware_data)

    def modify_bytes(self, offset: int, new_bytes: bytes, length: int = None, description: str = None):
        """
        Helper to modify firmware bytes at a given offset.
        Shows old bytes and new bytes for transparency.
        If length is not specified, uses len(new_bytes).
        """
        if length is None:
            length = len(new_bytes)
        old_bytes = self.firmware_data[offset:offset+length]
        self.firmware_data[offset:offset+length] = new_bytes
        desc = f" ({description})" if description else ""
        print(f"[MODIFY]{desc} Offset 0x{offset:X}: {old_bytes.hex()} -> {new_bytes.hex()}")

    def open_firmware(self, firmware_path: str) -> None:
        with open(firmware_path, 'rb') as f:
            self.firmware_data =  bytearray(f.read())

    def update_firmware_hashes(self):
        header_struct = list(self.get_header_struct())
        fw_size = header_struct[3]
        fw_data = self.firmware_data[FW_START_OFFSET : FW_START_OFFSET + fw_size]
        fw_crc = zlib.crc32(fw_data).to_bytes(4, 'big')
        header_struct[4] = fw_crc
        header_bytes = struct.pack(HEADER_STRUCT_FORMAT, *header_struct)
        # drop first 10 bytes for crc check
        header_bytes_past_crc = header_bytes[10:]
        header_crc = zlib.crc32(header_bytes_past_crc).to_bytes(4, 'big')
        header_struct[1] = header_crc
        complete_new_header = struct.pack(HEADER_STRUCT_FORMAT, *header_struct)
        self.firmware_data[HEADER_START_OFFSET: HEADER_START_OFFSET + HEADER_LEN] = complete_new_header
        print("Firmware hashes updated.")

    def get_header_struct(self):
        header_struct = struct.unpack_from(HEADER_STRUCT_FORMAT, self.firmware_data, HEADER_START_OFFSET)
        return header_struct

    def validate_firmware_version(self):
        # read offsets for version info and validate them
        header_struct = self.get_header_struct()

        # only valid against version 2.4.3.0 from March 21, 2023 (ts 1679431056), fail if anything else
        expected_version = b'\x02\x04\x03\x00'
        expected_timestamp = 1679431056
        if header_struct[7] != expected_version:
            raise ValueError(f"Firmware version mismatch. Expected {expected_version}, found {header_struct[7]}")
        if header_struct[5] != expected_timestamp:
            raise ValueError(f"Firmware timestamp mismatch. Expected {expected_timestamp}, found {header_struct[5]}")
        print("Firmware version validated successfully.")

    def disable_external_dns(self):
        """
        8.8.8.8 and 1.1.1.1 are hardcoded into the firmware in addition to dhcp provided dns.
        this disables them by replacing the instructions with noop.w (0xaff30000)
        """
        offset_a = 0x5971e
        offset_a_expected = b'\xf3\xf7\xc7\xfc'
        offset_b = 0x59730
        offset_b_expected = b'\xf3\xf7\xbe\xfc'
        patched_value = b'\xaf\xf3\x00\x00'

        if self.firmware_data[offset_a:offset_a+4] == offset_a_expected:
            self.modify_bytes(offset_a, patched_value, 4, "Disable external DNS A")
        elif self.firmware_data[offset_a:offset_a+4] == patched_value:
            print("External DNS A already disabled.")
        else:
            raise ValueError("Unexpected data at external DNS offset A.")

        if self.firmware_data[offset_b:offset_b+4] == offset_b_expected:
            self.modify_bytes(offset_b, patched_value, 4, "Disable external DNS B")
        elif self.firmware_data[offset_b:offset_b+4] == patched_value:
            print("External DNS B already disabled.")
        else:
            raise ValueError("Unexpected data at external DNS offset B.")
        print("External DNS disabled.")

    def replace_server_public_key(self, new_key_path: str):
        # need to validate the key, must be rsa 2048 bits, either pem or der format
        with open(new_key_path, "rb") as f:
            data = f.read()
            mykey = RSA.import_key(data)
            if mykey.size_in_bits() != 2048:
                raise ValueError("Server public key must be RSA 2048 bits.")
            pub_key = mykey.public_key()
            pub_key_der = pub_key.export_key(format='DER')
            # key stored in places
            offset_a = 0xa_a2c8
            offset_b = 0xa_cfc4
            size = len(pub_key_der)
            self.modify_bytes(offset_a, pub_key_der, size, "Replace server public key A")
            self.modify_bytes(offset_b, pub_key_der, size, "Replace server public key B")
            print("Server public key replaced successfully.")

    def replace_server_address(self, new_address: str, api_port : int = 5683, log_port : int = 1337):
        # we're replacing the string inplace to keep it simple.
        # in theory, we can put the string elsewhere and move all references
        log_offset = 0x86efc
        api_offset = 0x8765c
        max_len = 19 # excluding null terminator
        address_bytes = new_address.encode('utf-8')
        if len(address_bytes) > max_len:
            raise ValueError("New server address is too long. Max 19 characters.")
        # add null-terminator
        log_patch = address_bytes + b'\x00'
        api_patch = address_bytes + b'\x00'
        self.modify_bytes(log_offset, log_patch, len(log_patch), "Replace log server address (null-terminated)")
        self.modify_bytes(api_offset, api_patch, len(api_patch), "Replace API server address (null-terminated)")

        if api_port != 5683:
            port_hex = format(api_port, '04x')
            first_half_offset = 0x3_646a
            second_half_offset = 0x3_6470
            new_bytes1 = self.encode_movs(3, int(port_hex[0:2], 16))
            new_bytes2 = self.encode_movs(3, int(port_hex[2:], 16))
            self.modify_bytes(first_half_offset, new_bytes1, len(new_bytes1), "API port MOVS first half")
            self.modify_bytes(second_half_offset, new_bytes2, len(new_bytes2), "API port MOVS second half")

            # unsure what this one is for
            offset = 0x3_6426
            new_bytes = self.encode_movw_thumb2(3, api_port)
            self.modify_bytes(offset, new_bytes, len(new_bytes), "API port MOVW Thumb2")

            # these are only for logging messages
            log_msg_offsets = [0x3_6498, 0x3_64d6, 0x3_652a]
            for idx, offset in enumerate(log_msg_offsets):
                new_bytes = self.encode_movw_thumb2(2, api_port)
                self.modify_bytes(offset, new_bytes, len(new_bytes), f"API port MOVW Thumb2 log msg {idx+1}")
            print("API port replaced successfully.")

        if log_port != 1337:
            # one inst to rule them all. very convenient.
            new_bytes = self.encode_movw_thumb2(3, log_port)
            offset = 0x3_2a4c
            self.modify_bytes(offset, new_bytes, len(new_bytes), "Log port MOVW Thumb2")
            print("Log port replaced successfully.")
        print("Server address replaced successfully.")


    @staticmethod
    def encode_movw_thumb2(rd : int , imm16: int) -> bytes:
        """Encode Thumb-2 MOVW instruction, returns bytes in little-endian"""
        imm4 = (imm16 >> 12) & 0xF   # bits 15-12
        i    = (imm16 >> 11) & 0x1   # bit 11
        imm3 = (imm16 >> 8) & 0x7    # bits 10-8
        imm8 = imm16 & 0xFF          # bits 7-0

        # First halfword: 1111 0 i 10 0100 imm4
        hw1 = 0xF240 | (i << 10) | imm4

        # Second halfword: 0 imm3 Rd imm8
        hw2 = (imm3 << 12) | (rd << 8) | imm8

        # Return as little-endian bytes
        return bytes([hw1 & 0xFF, hw1 >> 8, hw2 & 0xFF, hw2 >> 8])

    @staticmethod
    def encode_movs(rd: int , imm8: int) -> bytes:
        """
        Encode 16-bit Thumb MOVS Rd, #imm8

        rd:   destination register (0-7)
        imm8: immediate value (0-255)

        Returns bytes in little-endian format
        """
        if rd > 7:
            raise ValueError("MOVS only supports R0-R7")
        if imm8 > 255:
            raise ValueError("MOVS only supports 0-255")

        # Encoding: 001 00 Rd(3) imm8(8)
        # Opcode 0x20 for R0, 0x21 for R1, etc.
        inst = 0x2000 | (rd << 8) | imm8

        return bytes([inst & 0xFF, inst >> 8])

def interactive_modify_firmware(firmware_path, server_public_key):
    print(f"Modifying firmware at: {firmware_path}")
    print(f"Using server public key: {server_public_key}")

    server_ip = input("Enter new server IP or domain: ").strip()
    spark_port = input("Enter new Spark port [5683]: ").strip()
    log_port = input("Enter new log port [1337]: ").strip()

    spark_port = int(spark_port) if spark_port else 5683
    log_port = int(log_port) if log_port else 1337

    updater = FirmwareUpdater(firmware_path)
    updater.validate_firmware_version()
    updater.replace_server_address(server_ip, api_port=spark_port, log_port=log_port)
    updater.replace_server_public_key(server_public_key)
    updater.disable_external_dns()
    updater.update_firmware_hashes()
    updater.write_firmware()
    print("Firmware modified successfully.")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Interactive firmware modifier.")
    parser.add_argument("firmware_path", type=str, help="Path to the firmware file to be modified.")
    parser.add_argument("server_public_key", type=str, help="Path to the new server public key (PEM/DER).")
    args = parser.parse_args()
    interactive_modify_firmware(args.firmware_path, args.server_public_key)
