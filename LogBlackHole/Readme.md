# Logging Black Hole
The pod sends up a continuous stream of log messages in clear text to their backend.
If the backend is unreachable, it will buffer messages on its sd card.  I'm not sure what happens when it fills up.
To avoid this, we create a fake logging server that'll accept the log messages.  We could save the messages, but right
now we just toss them.

Ideally log uploading is disabled entirely in firmware, but this will do for now.

## Protocol Specification
It's unsure if this is a some obscure standardized format or something custom.  
This also isn't a full specification, just what has reverse engineered so far and the minimum
that's needed for the pod to handshake and send messages so the card doesn't fill up.



### Handshake
1. pod connects to the server on tcp port 1337
2. pod sends the following payload, 12 byte number (bytes 0x1e - 0x35) is the stm32 unique id
```
0000   a4 65 70 72 6f 74 6f 63 72 61 77 64 70 61 72 74   .eprotocrawdpart
0010   67 73 65 73 73 69 6f 6e 63 64 65 76 78 18 00 11   gsessioncdevx.01
0020   22 33 44 55 66 77 88 99 0a 0b 0c 0d 0e 0f 00 01   23456789abcdef01
0030   02 03 04 05 06 07 67 76 65 72 73 69 6f 6e 61 32   234567gversiona2
```
3. server responds with the following payload
```
0000   a2 65 70 72 6f 74 6f 63 72 61 77 64 70 61 72 74   .eprotocrawdpart
0010   67 73 65 73 73 69 6f 6e                           gsession
```

### Batch Start
1. when pod is ready to upload a unit of log messages, (called a batch), it sends the following payload 
   * the 4 bytes a 0x0a - 0x0d is the batch id
```
0000   a4 65 70 72 6f 74 6f 63 72 61 77 64 70 61 72 74   .eprotocrawdpart
0010   65 62 61 74 63 68 62 69 64 1a 00 01 02 03 66 73   ebatchbid....>fs
0020   74 72 65 61 6d 5f                                 tream_
```
2. server responds with the following payload, inserting the same batch id at the end
    * the 4 bytes at 0x1a - 0x0d is the batch id
```
0000   a3 65 70 72 6f 74 6f 63 72 61 77 64 70 61 72 74   .eprotocrawdpart
0010   65 62 61 74 63 68 62 69 64 1a 00 01 02 03         ebatchbid.....
```

### Message Payload
The messages themselves have some header structure along with the log messages itself.
No work has been done to decode the header fields.

### Batch End
The pod indicates either the end of a file, or maybe a reset by sending a single `0xFF` payload

## Other Notes
* the reversed protocol implementation appears to be subtly wrong.  After the sdcard buffered data is sent, the continuous streaming seems to include a lot of padding that isn't present on wireshark dumps of the traffic with official servers.  Not sure why. This has the effect of amplifying the amount of bytes sent.
* Also looking at wireshark dumps, there appears to be cases where the handshake and batch start messages are skipped entirely.  Again, not sure why.