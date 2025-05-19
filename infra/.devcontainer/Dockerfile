FROM archlinux:base-devel-20250511.0.348143 AS arch

ARG GO_VERSION=1.24.3
ARG USER_NAME=arch

RUN useradd -m ${USER_NAME} 

RUN pacman -Sy --noconfirm nodejs npm git fzf openssh cmake && npm install -g aws-cdk

RUN curl -LO https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz \
    && rm go${GO_VERSION}.linux-amd64.tar.gz


RUN git clone https://github.com/neovim/neovim --branch stable --depth 1 \
    && cd neovim \
    && make CMAKE_BUILD_TYPE=RelWithDebInfo \
    && make install \
    && cd .. && rm -r neovim

ENV PATH="/usr/local/go/bin:${PATH}"

USER arch

