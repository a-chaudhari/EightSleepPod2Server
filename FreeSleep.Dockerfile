FROM node:24-bookworm

RUN apt-get update &&  \
    apt-get install -y git python3 python3-pip python3-numpy python3-scipy  \
            python3-pandas python3-cbor2 python3-watchdog python3-sentry-sdk

RUN mkdir -p /home/dac/venv/bin && chown -R 1000:1000 /home/dac/
RUN mkdir /deviceinfo/ && chown 1000:1000 /deviceinfo/
RUN ln -s /usr/bin/python3 /home/dac/venv/bin/python
WORKDIR /home/dac/
USER 1000:1000

RUN git clone -b docker_pod_2 https://github.com/a-chaudhari/free-sleep.git
WORKDIR /home/dac/free-sleep/server
EXPOSE 3000/tcp
RUN npm install

