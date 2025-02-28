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
RUN echo '[supervisord]\nnodaemon=true\nlogfile=/var/log/supervisord.log\nlogfile_maxbytes=0\n' > /etc/supervisor/conf.d/supervisord.conf && \
    mkdir /app/mosquitto_config && \
    touch /app/mosquitto_config/acl && \
    touch /app/mosquitto_config/passwd && \
    echo 'allow_anonymous false\nacl_file /data/mosquitto_config/acl\npassword_file /data/mosquitto_config/passwd\npid_file /data/mosquitto_config/pid\n' > /app/mosquitto_config/mosquitto.conf && \
    echo "moving to find3" && cd /build/find3/server/main && go build -v && \
    echo "moving main" && mv /build/find3/server/main /app/main && \
    echo "moving to ai" && cd /build/find3/server/ai && python3 -m pip install -r requirements.txt && \
    echo "moving ai" && mv /build/find3/server/ai /app/ai && \
    echo "removing go srces" && rm -rf /usr/local/work/src && \
    echo "purging packages" && apt-get remove -y --auto-remove git libc6-dev pkg-config g++ gcc && \
    echo "autoclean" && apt-get autoclean && \
    echo "clean" && apt-get clean && \
    echo "autoremove" && apt-get autoremove && \
    echo "rm trash" && rm -rf ~/.local/share/Trash/* && \
    echo "rm go" && rm -rf /usr/local/go* && \
    echo "rm perl" && rm -rf /usr/share/perl* && \
    echo "rm build" && rm -rf /build* && \
    echo "rm doc" && rm -rf /usr/share/doc*

# Copy the SSL certificates into the container
COPY fullchain.pem /etc/ssl/certs/fullchain.pem
COPY privkey.pem /etc/ssl/private/privkey.pem

# Copy the source code into the container
COPY . .

# Download dependencies
RUN go mod download

# Build the Go app
RUN go build -o main .

# Expose port 8003 to the outside world
EXPOSE 8003

# Set the working directory
WORKDIR /app

# Command to run the executable
CMD ["/app/startup.sh"]
