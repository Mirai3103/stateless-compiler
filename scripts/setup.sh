#!/usr/bin/env bash
# Exit on error, treat unset variables as an error, and propagate exit status through pipes
set -euo pipefail

echo "Starting full setup: nsjail and Alpine chroot with runtimes..."

# --- nsjail Installation ---
echo "Cloning nsjail from Mirai3103/nsjail..."
if [ -d "nsjail" ]; then
    echo "nsjail directory already exists. Skipping clone."
else
    git clone https://github.com/Mirai3103/nsjail.git
fi
cd nsjail

echo "Installing nsjail build and runtime dependencies..."
# Runtime dependencies (might already be on system, but good to ensure)
sudo apt-get -y update
sudo apt-get install -y \
    libc6 \
    libstdc++6 \
    libprotobuf32 \
    libnl-route-3-200

# Build dependencies
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
    protobuf-compiler

echo "Building nsjail..."
make clean
make -j$(nproc) # Use all available processors for faster build

echo "Installing nsjail to /usr/local/bin/nsjail..."
sudo install -m 755 nsjail /usr/local/bin/nsjail
echo "nsjail installation complete."
cd ..

echo "Cleaning up nsjail source directory..."
sudo rm -rf nsjail

# --- Alpine Chroot Setup ---
echo "Initializing Alpine chroot at /alpine..."
if [ ! -f "alpine-chroot-install" ]; then
    echo "Error: alpine-chroot-install script not found in the current directory."
    echo "Please download it from a trusted source (e.g., https://github.com/alpinelinux/alpine-chroot-install) or ensure it's available."
    exit 1
fi
sudo chmod +x alpine-chroot-install

if [ -d "/alpine/etc" ]; then # A simple check if chroot might already exist
    echo "Alpine chroot at /alpine seems to exist. Skipping installation."
else
    sudo bash -c "./alpine-chroot-install -d /alpine"
fi
echo "Alpine chroot base setup complete."

# --- Software Installation INSIDE Alpine Chroot ---
echo "Preparing script to install software inside Alpine chroot..."

# Create a script to run inside the chroot
# Using a temporary file on host, then copying it into chroot
# This is generally safer and more manageable than complex here-documents with chroot
CHROOT_SETUP_SCRIPT_HOST="/tmp/alpine_setup_script_host.sh"
CHROOT_SETUP_SCRIPT_GUEST="/tmp/alpine_setup_script_guest.sh" # Path inside chroot

cat <<EOF_CHROOT_SCRIPT > "${CHROOT_SETUP_SCRIPT_HOST}"
#!/bin/sh
# Exit on error within the chroot script
set -e

echo "[CHROOT] Updating apk and installing base packages..."
apk update
apk add --no-cache python3 go unzip curl bash openjdk17-jdk sudo git # Added git as it's often useful
echo "[CHROOT] Installing Node.js ..."
apk add --no-cache nodejs npm # Node.js and npm
echo "[CHROOT] Installing Rust..."
apk add --no-cache --virtual .rust-build-deps curl bash # Temporary build dependencies for Rust
# rustup installs to \$CARGO_HOME (default /root/.cargo) if run as root
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path
CARGO_HOME="/root/.cargo"
if [ -d "\$CARGO_HOME/bin" ]; then
    export PATH="\$CARGO_HOME/bin:\$PATH" # Make cargo/rustc available for verification step
    ln -sf "\$CARGO_HOME/bin/rustc" /usr/local/bin/rustc
    ln -sf "\$CARGO_HOME/bin/cargo" /usr/local/bin/cargo
    # You can add more symlinks here if needed: rustfmt, clippy, etc.
    echo "[CHROOT] Rustc and Cargo symlinked to /usr/local/bin/"
else
    echo "[CHROOT] Warning: Cargo bin directory not found at \$CARGO_HOME/bin after Rust installation."
fi
apk del .rust-build-deps # Clean up temporary dependencies

echo "[CHROOT] Verifying installations..."
echo "[CHROOT] Python version:"
python3 --version
echo "[CHROOT] Go version:"
go version
echo "[CHROOT] Bash version:"
bash --version

if command -v rustc >/dev/null 2>&1; then
    echo "[CHROOT] Rustc version:"
    rustc --version
else
    echo "[CHROOT] Rustc command not found in PATH for verification."
fi
if command -v cargo >/dev/null 2>&1; then
    echo "[CHROOT] Cargo version:"
    cargo --version
else
    echo "[CHROOT] Cargo command not found in PATH for verification."
fi
echo "[CHROOT] Java version:"
java -version

echo "[CHROOT] Cleaning up apk cache..."
apk cache clean

echo "[CHROOT] Alpine chroot software installation complete."
EOF_CHROOT_SCRIPT

# Ensure the host script is executable (though not strictly necessary as we cat it)
chmod +x "${CHROOT_SETUP_SCRIPT_HOST}"

echo "Copying setup script into Alpine chroot at /alpine${CHROOT_SETUP_SCRIPT_GUEST}..."
sudo cp "${CHROOT_SETUP_SCRIPT_HOST}" "/alpine${CHROOT_SETUP_SCRIPT_GUEST}"
sudo chmod +x "/alpine${CHROOT_SETUP_SCRIPT_GUEST}" # Make it executable inside chroot

echo "Running setup script inside Alpine chroot..."
# Execute the script inside the chroot. Using /bin/sh as it's guaranteed in Alpine.
sudo chroot /alpine /bin/sh "${CHROOT_SETUP_SCRIPT_GUEST}"

echo "Cleaning up temporary scripts..."
rm "${CHROOT_SETUP_SCRIPT_HOST}"
sudo rm "/alpine${CHROOT_SETUP_SCRIPT_GUEST}"

echo "-----------------------------------------------------"
echo "Setup finished!"
echo "nsjail is installed at /usr/local/bin/nsjail"
echo "Alpine chroot is at /alpine and configured with Python, Go, Bun, Rust, Java."
echo "-----------------------------------------------------"

