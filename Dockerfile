FROM ubuntu:18.04

ENV GOLANG_VERSION 1.11
ENV PATH="/usr/local/go/bin:/usr/local/work/bin:${PATH}"
ENV GOPATH /usr/local/work
ENV GO111MODULE=on

# Install dependencies
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
    wget git libc6-dev make pkg-config g++ gcc mosquitto-clients mosquitto \
    python3 python3-dev python3-pip python3-setuptools python3-wheel supervisor \
    libfreetype6-dev python3-matplotlib libopenblas-dev libblas-dev liblapack-dev gfortran && \
    python3 -m pip install Cython --install-option="--no-cython-compile" && \
    apt-get install --no-install-recommends -y python3-scipy python3-numpy && \
    mkdir /usr/local/work && \
    rm -rf /var/lib/apt/lists/* && \
    set -eux; \
    dpkgArch="$(dpkg --print-architecture)"; \
    case "${dpkgArch##*-}" in \
        amd64) goRelArch='linux-amd64'; goRelSha256='b3fcf280ff86558e0559e185b601c9eade0fd24c900b4c63cd14d1d38613e499' ;; \
    esac

# Create necessary directories and configuration files
RUN mkdir -p /app && \
    echo '[supervisord]\nnodaemon=true\nlogfile=/var/log/supervisord.log\nlogfile_maxbytes=0\n' > /etc/supervisor/conf.d/supervisord.conf && \
    mkdir /app/mosquitto_config && \
    touch /app/mosquitto_config/acl && \
    touch /app/mosquitto_config/passwd && \
    echo 'allow_anonymous false\nacl_file /data/mosquitto_config/acl\npassword_file /data/mosquitto_config/passwd\npid_file /data/mosquitto_config/pid\n' > /app/mosquitto_config/mosquitto.conf

# Copy the source code into the container
COPY . /build

# Build the Go application
RUN cd /build/LogicLoopServer/server/main && go build -v -o /app/main && \
    cd /build/LogicLoopServer/server/ai && python3 -m pip install -r requirements.txt && \
    mv /build/LogicLoopServer/server/ai /app/ai && \
    rm -rf /usr/local/work/src && \
    apt-get remove -y --auto-remove git libc6-dev pkg-config g++ gcc && \
    apt-get autoclean && apt-get clean && apt-get autoremove && \
    rm -rf ~/.local/share/Trash/* && rm -rf /usr/local/go* && rm -rf /usr/share/perl* && \
    rm -rf /build* && rm -rf /usr/share/doc*

# Copy the SSL certificates into the container
COPY fullchain.pem /etc/ssl/certs/fullchain.pem
COPY privkey.pem /etc/ssl/private/privkey.pem

# Set the working directory
WORKDIR /app

# Expose port 8003 to the outside world
EXPOSE 8003

# Command to run the executable
CMD ["/app/startup.sh"]
