#!/bin/bash
# Script to compile a Solidity contract and generate Go bindings
# Usage: ./generate-contract.sh ContractName

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <ContractName>"
    echo "Example: $0 TestEmitter"
    exit 1
fi

CONTRACT=$1
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/../testdata"

cd "$TESTDATA_DIR"

if [ ! -f "${CONTRACT}.sol" ]; then
    echo "Error: ${CONTRACT}.sol not found in $TESTDATA_DIR"
    exit 1
fi

echo "Compiling ${CONTRACT}.sol..."
solc --abi --bin --overwrite -o . ${CONTRACT}.sol

echo "Generating Go bindings..."
abigen --abi=${CONTRACT}.abi \
       --bin=${CONTRACT}.bin \
       --pkg=testdata \
       --type=${CONTRACT} \
       --out=${CONTRACT}.go

echo "Cleaning up intermediate files..."
rm ${CONTRACT}.abi ${CONTRACT}.bin

echo "✓ Successfully generated ${CONTRACT}.go"
