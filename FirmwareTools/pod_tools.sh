#!/bin/bash

# Bash script to dump or write pod firmware using openocd
# Usage:
#   ./pod_tools.sh dump <output_file>
#   ./pod_tools.sh write <firmware_file>
#   ./pod_tools.sh test
#   ./pod_tools.sh console [device]

set -e

INTERFACE_CFG="interface/stlink.cfg"
TARGET_CFG="target/stm32f4x.cfg"

function check_pod_status() {
    set +e  # Disable immediate exit on error for this function
    echo "Checking pod connection..."
    openocd -f "$INTERFACE_CFG" -f "$TARGET_CFG" -c "init; shutdown" > /dev/null 2>&1
    local status=$?
    set -e  # Re-enable immediate exit on error
    if [ $status -eq 0 ]; then
        echo "Pod is connected and responsive."
    else
        echo "Pod is not responding. Please check the connection."
        exit 1
    fi
}

function dump_pod_firmware_to_file() {
    local output_file="$1"
    echo "Dumping pod firmware to $output_file..."
    openocd -f "$INTERFACE_CFG" -f "$TARGET_CFG" -c "init; flash read_bank 0 $output_file; shutdown"
    if [ $? -eq 0 ]; then
        echo "Firmware successfully dumped to $output_file."
    else
        echo "Failed to dump firmware. Please check the connection and output file path."
        exit 1
    fi
}

function write_firmware_to_pod() {
    local firmware_file="$1"
    echo "Writing firmware $firmware_file to pod..."
    openocd -f "$INTERFACE_CFG" -f "$TARGET_CFG" -c "init; program $firmware_file 0x08000000 verify reset; shutdown"
    if [ $? -eq 0 ]; then
        echo "Firmware successfully written to the pod."
    else
        echo "Failed to write firmware. Please check the connection and firmware file."
        exit 1
    fi
}

function console() {
    local dev_path="$1"
    if [ -z "$dev_path" ]; then
        dev_path="/dev/ttyUSB0"
    fi
    echo "Opening console on $dev_path..."
    picocom "$dev_path" -b 115200 --imap lfcrlf
}

if [ "$#" -lt 1 ]; then
    echo "Usage: $0 dump <output_file>"
    echo "       $0 write <firmware_file>"
    echo "       $0 test"
    echo "       $0 console [device]"
    exit 1
fi

COMMAND="$1"
FILE_ARG="$2"

case "$COMMAND" in
    dump)
        if [ -z "$FILE_ARG" ]; then
            echo "Usage: $0 dump <output_file>"
            exit 1
        fi
        dump_pod_firmware_to_file "$FILE_ARG"
        ;;
    write)
        if [ -z "$FILE_ARG" ]; then
            echo "Usage: $0 write <firmware_file>"
            exit 1
        fi
        write_firmware_to_pod "$FILE_ARG"
        ;;
    test)
        check_pod_status
        ;;
    console)
        console "$FILE_ARG"
        ;;
    *)
        echo "Unknown command: $COMMAND"
        echo "Usage: $0 dump <output_file>"
        echo "       $0 write <firmware_file>"
        echo "       $0 test"
        echo "       $0 console [device]"
        exit 1
        ;;
esac
