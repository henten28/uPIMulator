FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONPATH="/root/upmem_linker/src:$PYTHONPATH"

RUN chmod 1777 /tmp

RUN apt update
RUN apt install -y git
RUN apt install -y wget
RUN apt install -y vim
RUN apt install -y tmux
RUN apt install -y mlocate
RUN apt install -y cmake
RUN apt install -y ninja-build
RUN apt install -y pkg-config
RUN apt install -y libnuma-dev
RUN apt install -y libelf-dev
RUN apt install -y flex

# Python 3.10
RUN apt update
RUN apt upgrade -y
RUN apt install -y software-properties-common
RUN apt-get update && \
    apt-get install -y --no-install-recommends python3 python3-pip python3-venv && \
    rm -rf /var/lib/apt/lists/*
RUN add-apt-repository ppa:deadsnakes/ppa
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-pip python3-venv && rm -rf /var/lib/apt/lists/*
RUN apt install -y python3-pip

# UPMEM LLVM
WORKDIR /root
# SKIPPED TEMPORARILY: RUN git clone https://github.com/upmem/llvm-project.git
RUN mkdir -p /root/llvm-project/build
WORKDIR /root/llvm-project/build
RUN apt-get update && apt-get install -y --no-install-recommends build-essential g++ cmake ninja-build && rm -rf /var/lib/apt/lists/* && cmake -G Ninja /root/llvm-project/llvm -DLLVM_ENABLE_PROJECTS="clang"
RUN cmake build .

# UPMEM SDK
WORKDIR /root
RUN pip3 install gdown
RUN gdown 1KWTkZnLWEMZ2X6RTxp9JSYEdm_3jogmV -O upmem-2023.2.0-Linux-x86_64.tar.gz
RUN tar -zxvf upmem-2023.2.0-Linux-x86_64.tar.gz
RUN echo "source /root/upmem-2023.2.0-Linux-x86_64/upmem_env.sh" > /root/.bashrc

WORKDIR /root/upmem_compiler
