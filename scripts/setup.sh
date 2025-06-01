#  install
git clone https://github.com/Mirai3103/nsjail.git
cd nsjail
sudo apt-get -y update && apt-get install -y \
    libc6 \
    libstdc++6 \
    libprotobuf32 \
    libnl-route-3-200
sudo apt-get install -y \
    autoconf \
    bison \
    flex \
    gcc \
    g++ \
    git \
    libprotobuf-dev \
    libnl-route-3-dev \
    libtool \
    make \
    pkg-config \
    protobuf-compiler \
    pkg-config
make clean && make

sudo install -m 755 nsjail /usr/local/bin/nsjail
refreshEnv(){
    if [ -n "$BASH_VERSION" ]; then
        source ~/.bashrc
    elif [ -n "$ZSH_VERSION" ]; then
        source ~/.zshrc
    fi
}
# install python
sudo apt-get install -y python3
# install golang
# check if go is already installed
if command -v go &>/dev/null; then
    echo "Go is already installed."
else
    echo "Installing Go..."
    wget https://go.dev/dl/go1.24.3.linux-amd64.tar.gz &&
        tar -xzf go1.24.3.linux-amd64.tar.gz -C /usr/local &&
        rm go1.24.3.linux-amd64.tar.gz

    if [ -n "$BASH_VERSION" ]; then
        echo "export PATH=/usr/local/go/bin:\$PATH" >>~/.bashrc
        echo "export GOPATH=/go" >>~/.bashrc
        echo "export GOROOT=/usr/local/go" >>~/.bashrc
        source ~/.bashrc
    elif [ -n "$ZSH_VERSION" ]; then
        echo "export PATH=/usr/local/go/bin:\$PATH" >>~/.zshrc
        echo "export GOPATH=/go" >>~/.zshrc
        echo "export GOROOT=/usr/local/go" >>~/.zshrc
        source ~/.zshrc
    fi
fi

# install bun (a javascript/typescript runtime), no need to install nodejs
if command -v bun &>/dev/null; then
    echo "Bun is already installed."
else
    echo "Installing Bun..."
    curl -fsSL https://bun.sh/install | bash
    refreshEnv  
fi

# install rust
if command -v cargo &>/dev/null; then
    echo "Rust is already installed."
else
    echo "Installing Rust..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y 
    refreshEnv
fi

# install java 17
sudo apt install openjdk-17-jdk -y

