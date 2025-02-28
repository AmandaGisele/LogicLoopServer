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

# Install Go
RUN wget -q -O /tmp/go${GOLANG_VERSION}.linux-amd64.tar.gz https://dl.google.com/go/go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf /tmp/go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    rm /tmp/go${GOLANG_VERSION}.linux-amd64.tar.gz

# Create necessary directories and configuration files
RUN mkdir -p /app && \
    echo '[supervisord]\nnodaemon=true\nlogfile=/var/log/supervisord.log\nlogfile_maxbytes=0\n' > /etc/supervisor/conf.d/supervisord.conf && \
    mkdir /app/mosquitto_config && \
    touch /app/mosquitto_config/acl && \
    touch /app/mosquitto_config/passwd && \
    echo 'allow_anonymous false\nacl_file /data/mosquitto_config/acl\npassword_file /data/mosquitto_config/passwd\npid_file /data/mosquitto_config/pid\n' > /app/mosquitto_config/mosquitto.conf

# Copy the source code into the container
COPY . /build

# List the contents of the /build directory for debugging
RUN ls -R /build

# Build the Go application with debug output
RUN cd /build/server/main && go build -v -o /app/main || { echo 'Go build failed'; exit 1; }

# Install Python dependencies
RUN cd /build/server/ai && python3 -m pip install -r requirements.txt

# Move the AI server to the app directory
RUN mv /build/server/ai /app/ai

# Clean up unnecessary files
RUN rm -rf /usr/local/work/src /build /usr/share/doc && \
    apt-get remove -y --auto-remove git libc6-dev pkg-config g++ gcc && \
    apt-get autoclean && apt-get clean && apt-get autoremove && \
    rm -rf ~/.local/share/Trash/* /usr/local/go* /usr/share/perl*

# Copy the SSL certificates into the container
COPY cert.pem /etc/ssl/cert.pem
COPY fullchain.pem /etc/ssl/fullchain.pem
COPY privkey.pem /etc/ssl/privkey.pem

# Verify the SSL certificates are copied correctly
RUN ls -l /etc/ssl/fullchain.pem && ls -l /etc/ssl/privkey.pem

# Set the working directory
WORKDIR /app

# Expose port 8003 to the outside world
EXPOSE 8003

# Command to run the executable
CMD ["/app/startup.sh"]
