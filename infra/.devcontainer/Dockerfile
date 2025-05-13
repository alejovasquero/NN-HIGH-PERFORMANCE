FROM archlinux:base-devel-20250511.0.348143 AS arch

ARG GO_VERSION=1.24.3
ARG USER_NAME=arch

RUN useradd -m ${USER_NAME} 

RUN pacman -Sy --noconfirm nodejs npm git fzf openssh && npm install -g aws-cdk

RUN curl -LO https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz \
    && rm go${GO_VERSION}.linux-amd64.tar.gz \
    && echo "export PATH=${PATH}:/usr/local/go/bin" >> /home/${USER_NAME}/.bash_profile

ENV PATH="/usr/local/go/bin:${PATH}"

USER arch

