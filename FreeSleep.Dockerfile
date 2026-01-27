FROM node:24-bookworm

RUN apt-get update && apt-get install -y git
RUN git clone https://github.com/throwaway31265/free-sleep.git
WORKDIR /free-sleep/server
RUN npm install

