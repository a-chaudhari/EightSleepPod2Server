## Setup Steps
1. perform the binary patching steps (to be written...)
1. create a 2048 rsa private key and save to `server_priv_key.pem` in the root of this project
```
openssl genrsa -out server_priv_key.pem 2048
```
3. `docker compose up -d`


## Notes to self for later...
### dns binary patching
the pod has 2 hardcoded dns servers in addition to the dhcp provided one. 1.1.1.1 and 8.8.8.8.  We can disable the 2 hardcoded by binary patching
flash dump offset 0x0005971e patch the 4 bytes to 0xaff30000 which is noop.w 
do the same for offset 0x00059730