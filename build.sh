#!/bin/bash
set -e

echo "==============================="
echo "  PeerFlow — Cross-Platform Build"
echo "==============================="
echo ""

# Clean previous builds
rm -rf build/bin
mkdir -p build/bin

# Build for Windows (amd64)
echo "🔨 Building for Windows (amd64)..."
wails build -clean -platform windows/amd64 -trimpath
echo "✅ Windows build complete"
echo ""

# Build for macOS Apple Silicon (arm64)
echo "🍎 Building for macOS Apple Silicon (arm64)..."
wails build -clean -platform darwin/arm64 -trimpath
echo "✅ macOS ARM64 build complete"
echo ""

echo "==============================="
echo "  All builds completed!"
echo "  Output: build/bin/"
echo "==============================="
ls -lh build/bin/
