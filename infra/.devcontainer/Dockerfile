FROM archlinux:base-devel-20250511.0.348143 AS arch

ARG NODE_VERSION=22.0.0
ARG GO_VERSION=1.24.3
ARG USER_NAME=arch

RUN useradd -m ${USER_NAME} && echo "${USER_NAME} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

RUN pacman -Sy --noconfirm git fzf openssh cmake unzip less python python-pip docker gum socat


ENV NVM_DIR=/usr/local/nvm
RUN mkdir $NVM_DIR

RUN curl --silent -o- https://raw.githubusercontent.com/creationix/nvm/v0.31.2/install.sh | bash
RUN bash -c "source $NVM_DIR/nvm.sh && nvm install $NODE_VERSION"


RUN curl -LO https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz \
    && rm go${GO_VERSION}.linux-amd64.tar.gz

RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install

RUN git clone https://github.com/neovim/neovim --branch stable --depth 1 \
    && cd neovim \
    && make CMAKE_BUILD_TYPE=RelWithDebInfo \
    && make install \
    && cd .. && rm -r neovim

ENV PATH="/usr/local/go/bin:$NVM_DIR/versions/node/v$NODE_VERSION/bin:$PATH:${PATH}"
ENV NODE_PATH $NVM_DIR/v$NODE_VERSION/lib/node_modules
ENV USERNAME arch

RUN node -v && npm -v
RUN npm install -g aws-cdk

USER arch

