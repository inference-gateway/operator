FROM mcr.microsoft.com/devcontainers/go:1-1.24-bookworm

ENV GOPATH=/home/vscode/go \
    PATH="/home/vscode/go/bin:${PATH}"

# Tool Versions
ENV TASK_VERSION=v3.43.3 \
    CTLPTL_VERSION=0.8.40 \
    K3D_VERSION=v5.8.3 \
    KUBECTL_VERSION=v1.33.0 \
    GOLANG_CI_LINT_VERSION=v2.1.5 \
    GITHUB_CLI_VERSION=2.72.0 \
    SEMANTIC_RELEASE_VERSION=v24.2.3 \
    KUBEBUILDER_VERSION=v4.5.2

# Install common
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
RUN apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg nodejs

# Install task
RUN curl -fsSL https://taskfile.dev/install.sh | sh -s -- -b /usr/local/bin ${TASK_VERSION}

# Install ctlptl
RUN curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v${CTLPTL_VERSION}/ctlptl.${CTLPTL_VERSION}.linux.$(dpkg --print-architecture).tar.gz | tar -xzv -C /usr/local/bin ctlptl

# Install k3d
RUN curl -fsSL https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${K3D_VERSION} bash

# Install kubectl
RUN curl -L "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/$(dpkg --print-architecture)/kubectl" -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install golangci-lint
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANG_CI_LINT_VERSION}

# Install gh
RUN curl -fsSL https://github.com/cli/cli/releases/download/v${GITHUB_CLI_VERSION}/gh_${GITHUB_CLI_VERSION}_linux_$(dpkg --print-architecture).tar.gz | tar -xzv -C /usr/local/bin --strip-components=2 gh_${GITHUB_CLI_VERSION}_linux_$(dpkg --print-architecture)/bin/gh

# Install kubebuilder
RUN curl -L -o kubebuilder https://github.com/kubernetes-sigs/kubebuilder/releases/download/${KUBEBUILDER_VERSION}/kubebuilder_$(go env GOOS)_$(go env GOARCH) && \
    chmod +x kubebuilder && \
    mv kubebuilder /usr/local/bin/

# Install semantic-release
RUN npm install -g semantic-release@${SEMANTIC_RELEASE_VERSION} \
    @semantic-release/changelog \
    @semantic-release/git \
    @semantic-release/github \
    @semantic-release/exec \
    conventional-changelog-cli \
    conventional-changelog-conventionalcommits

# Install powerlevel10k theme for zsh
RUN git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/vscode/.oh-my-zsh/custom/themes/powerlevel10k
RUN echo 'source /home/vscode/.oh-my-zsh/custom/themes/powerlevel10k/powerlevel10k.zsh-theme' >> /home/vscode/.zshrc
RUN echo 'POWERLEVEL9K_DISABLE_CONFIGURATION_WIZARD=true' >> /home/vscode/.zshrc

# Setup Go environment for vscode user
RUN mkdir -p /home/vscode/go && chown -R vscode:vscode /home/vscode/go
RUN echo 'export GOPATH=/home/vscode/go' >> /home/vscode/.zshrc
RUN echo 'export PATH=$GOPATH/bin:$PATH' >> /home/vscode/.zshrc

# Auto-completions
RUN echo '# Enable autocompletion' >> /home/vscode/.zshrc
RUN echo 'autoload -Uz compinit && compinit' >> /home/vscode/.zshrc
RUN echo 'source <(kubectl completion zsh)' >> /home/vscode/.zshrc
RUN echo 'source <(k3d completion zsh)' >> /home/vscode/.zshrc
RUN echo 'source <(task --completion zsh)' >> /home/vscode/.zshrc
RUN echo 'source <(ctlptl completion zsh)' >> /home/vscode/.zshrc
RUN echo 'source <(gh completion -s zsh)' >> /home/vscode/.zshrc
RUN echo 'source <(kubebuilder completion zsh)' >> /home/vscode/.zshrc

WORKDIR /workspaces/operator
USER vscode
