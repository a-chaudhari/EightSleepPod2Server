#!/usr/bin/env python3
"""
Eight Sleep Soft-AP WiFi Configuration Tool
Connect to the Eight Sleep's AP (eight-xxxx) before running this script.

Requires: pip install pycryptodome
"""

import socket
import json
import binascii
from Crypto.PublicKey import RSA
from Crypto.Cipher import PKCS1_v1_5

DEVICE_IP = "192.168.0.1"
DEVICE_PORT = 5609
TIMEOUT = 10


def send_command(command: str, data: str = "") -> str:
    """Send a command to the Eight Sleep and return the response."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(TIMEOUT)
        sock.connect((DEVICE_IP, DEVICE_PORT))

        payload = f"{command}\n{len(data)}\n\n{data}"
        sock.sendall(payload.encode())

        response = b""
        while True:
            try:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                response += chunk
            except socket.timeout:
                break

        return response.decode(errors='ignore')


def parse_response(response: str) -> dict:
    """Parse the Soft-AP response format."""
    lines = response.strip().split('\n')
    if len(lines) < 1:
        return {"error": "Invalid response"}

    for i, line in enumerate(lines):
        try:
            return json.loads(line)
        except json.JSONDecodeError:
            return {"raw": line}

    return {"raw": response}


def get_device_id() -> str:
    """Get the device ID."""
    response = send_command("device-id")
    data = parse_response(response)
    return data.get("id", "Unknown")


def get_public_key() -> bytes:
    """Retrieve the device's RSA public key."""
    response = send_command("public-key")
    data = parse_response(response)

    # Key is returned as hex string
    hex_key = data.get("b", "")
    if not hex_key:
        raise ValueError("Failed to retrieve public key")

    # Convert hex to bytes
    return binascii.unhexlify(hex_key)

def parse_der_public_key(der_bytes: bytes) -> RSA.RsaKey:
    """
    Parse Particle's DER-encoded RSA-1024 public key.
    Format: Standard DER SubjectPublicKeyInfo, zero-padded.
    """
    # Strip any trailing zeros (padding)
    der_bytes = der_bytes.rstrip(b'\x00')

    # Try standard import (works if it's proper DER)
    try:
        return RSA.import_key(der_bytes)
    except (ValueError, IndexError) as e:
        print(f"  Standard import failed: {e}")

    # Manual DER parsing for Particle's format
    # The structure is: SEQUENCE { SEQUENCE { OID, NULL }, BIT STRING { modulus, exponent } }

    # For RSA-1024, look for 128-byte modulus
    # Skip DER headers and find the actual key data

    idx = 0

    # Skip outer SEQUENCE tag and length
    if der_bytes[idx] == 0x30:  # SEQUENCE
        idx += 1
        if der_bytes[idx] & 0x80:  # Long form length
            len_bytes = der_bytes[idx] & 0x7F
            idx += 1 + len_bytes
        else:
            idx += 1

    # Skip algorithm identifier SEQUENCE
    if idx < len(der_bytes) and der_bytes[idx] == 0x30:
        idx += 1
        if der_bytes[idx] & 0x80:
            len_bytes = der_bytes[idx] & 0x7F
            skip = int.from_bytes(der_bytes[idx+1:idx+1+len_bytes], 'big')
            idx += 1 + len_bytes + skip
        else:
            idx += 1 + der_bytes[idx]

    # Skip BIT STRING tag
    if idx < len(der_bytes) and der_bytes[idx] == 0x03:
        idx += 1
        if der_bytes[idx] & 0x80:
            len_bytes = der_bytes[idx] & 0x7F
            idx += 1 + len_bytes
        else:
            idx += 1
        idx += 1  # Skip unused bits byte

    # Now we should be at the inner SEQUENCE containing modulus and exponent
    if idx < len(der_bytes) and der_bytes[idx] == 0x30:
        idx += 1
        if der_bytes[idx] & 0x80:
            len_bytes = der_bytes[idx] & 0x7F
            idx += 1 + len_bytes
        else:
            idx += 1

    # Read INTEGER (modulus)
    if idx < len(der_bytes) and der_bytes[idx] == 0x02:
        idx += 1
        if der_bytes[idx] & 0x80:
            len_bytes = der_bytes[idx] & 0x7F
            mod_len = int.from_bytes(der_bytes[idx+1:idx+1+len_bytes], 'big')
            idx += 1 + len_bytes
        else:
            mod_len = der_bytes[idx]
            idx += 1

        modulus = int.from_bytes(der_bytes[idx:idx+mod_len], 'big')
        idx += mod_len

        # Read INTEGER (exponent)
        if idx < len(der_bytes) and der_bytes[idx] == 0x02:
            idx += 1
            if der_bytes[idx] & 0x80:
                len_bytes = der_bytes[idx] & 0x7F
                exp_len = int.from_bytes(der_bytes[idx+1:idx+1+len_bytes], 'big')
                idx += 1 + len_bytes
            else:
                exp_len = der_bytes[idx]
                idx += 1

            exponent = int.from_bytes(der_bytes[idx:idx+exp_len], 'big')

            return RSA.construct((modulus, exponent))

    raise ValueError("Could not parse DER public key")


def encrypt_password(password: str, public_key_der: bytes) -> str:
    """Encrypt the WiFi password using RSA-1024 with PKCS#1 v1.5 padding."""
    # Parse the public key
    rsa_key = parse_der_public_key(public_key_der)

    # Create cipher with PKCS#1 v1.5 padding
    cipher = PKCS1_v1_5.new(rsa_key)

    # Encrypt the password
    encrypted = cipher.encrypt(password.encode('utf-8'))

    # Return as hex string
    return binascii.hexlify(encrypted).decode('ascii')


def scan_wifi() -> list:
    """Scan for available WiFi networks, deduplicate by SSID, keep strongest signal."""
    print("\nScanning for WiFi networks...")
    response = send_command("scan-ap")
    data = parse_response(response)

    if isinstance(data, dict) and "scans" in data:
        networks = data["scans"]
    elif isinstance(data, list):
        networks = data
    else:
        return []

    # Deduplicate by SSID, keeping strongest signal
    unique_networks = {}
    for net in networks:
        ssid = net.get("ssid", "")
        if not ssid:
            continue

        rssi = net.get("rssi", -100)
        if ssid not in unique_networks or rssi > unique_networks[ssid].get("rssi", -100):
            unique_networks[ssid] = net

    # Sort by RSSI descending
    return sorted(
        unique_networks.values(),
        key=lambda x: x.get("rssi", -100),
        reverse=True
    )


def display_networks(networks: list) -> None:
    """Display available networks in a formatted table."""
    if not networks:
        print("No networks found.")
        return

    print("\n" + "=" * 65)
    print(f"{'#':<4} {'SSID':<32} {'Security':<18} {'Signal'}")
    print("=" * 65)

    for i, net in enumerate(networks, 1):
        ssid = net.get("ssid", "Hidden")
        sec_value = net.get("sec", 0)
        security = get_security_string(sec_value)
        rssi = net.get("rssi", -100)

        if rssi >= -50:
            bars = "████"
        elif rssi >= -60:
            bars = "███░"
        elif rssi >= -70:
            bars = "██░░"
        elif rssi >= -80:
            bars = "█░░░"
        else:
            bars = "░░░░"

        print(f"{i:<4} {ssid:<32} {security:<18} {bars} {rssi} dBm")

    print("=" * 65)


def get_security_string(sec_value: int) -> str:
    """Convert WICED security bitmask to readable string."""
    KNOWN_VALUES = {
        0: "Open",
        1: "WEP",
        2: "WEP (Shared)",
        4194308: "WPA2-PSK (AES)",
        2097155: "WPA-PSK (TKIP)",
        4194307: "WPA-PSK (AES)",
        6291460: "WPA2-PSK (Mixed)",
        2097156: "WPA2-PSK (TKIP)",
        6291459: "WPA-PSK (Mixed)",
    }

    if sec_value in KNOWN_VALUES:
        return KNOWN_VALUES[sec_value]

    sec_type = sec_value & 0xFF
    cipher = (sec_value >> 20) & 0xF

    type_map = {0: "Open", 1: "WEP", 2: "WEP", 3: "WPA", 4: "WPA2", 5: "WPA-Ent", 6: "WPA2-Ent"}
    type_str = type_map.get(sec_type, f"Type{sec_type}")

    cipher_parts = []
    if cipher & 0x2:
        cipher_parts.append("TKIP")
    if cipher & 0x4:
        cipher_parts.append("AES")

    if cipher_parts:
        return f"{type_str} ({'/'.join(cipher_parts)})"
    return type_str


def configure_wifi(ssid: str, encrypted_password: str, security: int, channel: int) -> bool:
    """Configure WiFi credentials on the device."""
    config = {
        "ssid": ssid,
        "pwd": encrypted_password,  # Now RSA encrypted
        "sec": security,
        "ch": channel
    }

    response = send_command("configure-ap", json.dumps(config, separators=(',', ':')))
    data = parse_response(response)
    return data.get("r", -1) == 0


def connect_wifi() -> bool:
    """Tell the device to connect to the configured network."""
    response = send_command("connect-ap", json.dumps({"idx": 0}))
    data = parse_response(response)
    return data.get("r", -1) == 0


def main():
    print("=" * 65)
    print("  Eight Sleep WiFi Configuration Tool")
    print("=" * 65)
    print("\nMake sure you're connected to the Eight Sleep's AP (eight-xxxxx)")

    # Get device ID
    try:
        device_id = get_device_id()
        print(f"\nConnected to device: {device_id}")
    except Exception as e:
        print(f"\nError connecting to device: {e}")
        print("Make sure you're connected to the Eight Sleep's WiFi network.")
        return

    # Get public key for password encryption
    try:
        print("Retrieving device public key...")
        public_key = get_public_key()
        print(f"Public key retrieved ({len(public_key)} bytes)")
    except Exception as e:
        print(f"Error retrieving public key: {e}")
        return

    # Scan for networks
    try:
        networks = scan_wifi()
    except Exception as e:
        print(f"Error scanning: {e}")
        return

    if not networks:
        print("No networks found. Try again.")
        return

    display_networks(networks)

    # Select network
    while True:
        try:
            choice = input("\nSelect network number (or 'q' to quit): ").strip()
            if choice.lower() == 'q':
                return

            idx = int(choice) - 1
            if 0 <= idx < len(networks):
                selected = networks[idx]
                break
            print("Invalid selection. Try again.")
        except ValueError:
            print("Please enter a number.")

    ssid = selected.get("ssid", "")
    security = selected.get("sec", 0)
    channel = selected.get("ch", 0)

    print(f"\nSelected: {ssid}")
    print(f"Security: {get_security_string(security)}")

    # Get and encrypt password if needed
    encrypted_password = ""
    if security != 0:  # Not open network
        password = input("Enter WiFi password: ").strip()

        print("Encrypting password...")
        try:
            encrypted_password = encrypt_password(password, public_key)
        except Exception as e:
            print(f"Error encrypting password: {e}")
            return

    # Configure and connect
    print("\nConfiguring WiFi...")
    if configure_wifi(ssid, encrypted_password, security, channel):
        print("Configuration successful!")
    else:
        print("Configuration failed!")
        return

    print("Connecting to network...")
    if connect_wifi():
        print("\n✓ Connection initiated!")
        print("  The device will now disconnect from Soft-AP mode")
        print("  and attempt to connect to the configured network.")
    else:
        print("Connection command failed!")


if __name__ == "__main__":
    main()
