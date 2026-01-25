## dns binary patching
the pod has 2 hardcoded dns servers in addition to the dhcp provided one. 1.1.1.1 and 8.8.8.8.  We can disable the 2 hardcoded by binary patching
file dump offset 0x0005971e patch the 4 bytes to 0xaff30000 which is noop.w 
do the same for offset 0x00059730