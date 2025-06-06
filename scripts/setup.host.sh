#!/usr/bin/env bash
set -euo pipefail

echo "=== Starting full setup: nsjail and Alpine chroot with runtimes ==="

# --- nsjail Installation ---
echo "Cloning nsjail from Mirai3103/nsjail..."
if [ -d "nsjail" ]; then
    echo "nsjail directory already exists. Skipping clone."
else
    git clone https://github.com/Mirai3103/nsjail.git
  
fi
cd nsjail
  git checkout export-stats
echo "Installing nsjail build and runtime dependencies..."
sudo apt-get -y update
sudo apt-get install -y \
    libc6 bash libstdc++6 libnl-route-3-200 libprotobuf-dev \
    autoconf bison flex gcc g++ git \
     libnl-route-3-dev libtool make pkg-config protobuf-compiler

echo "Building nsjail..."
make clean
make -j"$(nproc)"

echo "Installing nsjail to /usr/local/bin/nsjail..."
sudo install -m 755 nsjail /usr/local/bin/nsjail
cd ..

echo "Cleaning up nsjail source directory..."
# sudo apt-get purge -y \
#     autoconf bison flex  \
#     libprotobuf-dev libnl-route-3-dev libtool \
#     make pkg-config protobuf-compiler
sudo rm -rf nsjail

# --- Setup sandbox directory ---
SANDBOXBASEDIR=${RUNNER_RUNNER_SANDBOXBASEDIR:-/sandbox}
sudo mkdir -p "$SANDBOXBASEDIR"
chmod -R 755 "$SANDBOXBASEDIR"

# --- Prepare system-wide PATH profile ---
echo "Configuring system-wide PATH for installed languages..."
sudo tee /etc/profile.d/init.sh > /dev/null <<'EOF'
#!/bin/sh
export PATH="$PATH:/usr/local/cargo/bin"
export PATH="$PATH:/usr/local/go/bin"
EOF
sudo chmod +x /etc/profile.d/init.sh

# --- Node.js Installation ---
echo "Installing Node.js..."
curl -sL https://deb.nodesource.com/setup_22.x -o nodesource_setup.sh
sudo bash nodesource_setup.sh
rm -f nodesource_setup.sh
sudo apt-get install -y nodejs

# --- Python Installation ---
echo "Installing Python..."
sudo apt-get install -y python3 python3-pip

# --- Rust Installation (global) ---
echo "Installing Rust (global)..."
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    sudo env CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup sh -s -- -y --no-modify-path --default-toolchain stable

# Set default toolchain
/usr/local/cargo/bin/rustup default stable

# --- Go Installation ---
echo "Installing Go..."
curl -sL https://go.dev/dl/go1.24.4.linux-amd64.tar.gz | sudo tar -C /usr/local -xz

# --- Java Installation ---
echo "Installing OpenJDK 17..."
sudo apt-get install -y openjdk-17-jdk

# --- Final Cleanup ---
echo "Cleaning up apt cache..."
sudo apt-get autoremove -y
sudo apt-get clean
sudo rm -rf /var/lib/apt/lists/*

echo "=== Setup complete. All tools installed successfully. ==="
