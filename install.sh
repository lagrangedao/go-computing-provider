#!/bin/bash

# Check Python version and install if necessary
if ! command -v python3 &> /dev/null; then
    echo "Installing Python 3..."
    sudo apt-get update
    sudo apt-get install -y python3
fi

# Check if pip is installed and install if necessary
if ! command -v pip3 &> /dev/null; then
    echo "Installing pip..."
    sudo apt-get install -y python3-pip
else
    echo "pip is installed."
fi

# Check Python version
python_version=$(python3 --version | awk '{print $2}')
required_version="3.8"

if [ "$(printf '%s\n' "$required_version" "$python_version" | sort -V | head -n 1)" != "$required_version" ]; then
    echo "Error: Python version must be greater than or equal to $required_version"
    exit 1
else
    echo "Python version $python_version is compatible."
fi


# Check if the environment variable is set
if [ -z "$CP_PATH" ]; then
    echo "Error: CP_PATH is not set. Please set it using: export CP_PATH=xxx"
    exit 1
else
    echo "CP_PATH is set to $CP_PATH"
fi

# Check if the directory exists, create if not
if [ ! -d "$CP_PATH/inference-model" ]; then
    mkdir -p "$CP_PATH/inference-model"

    # Clone the repository and switch to the specified branch
    git clone https://github.com/lagrangedao/api-inference-community.git "$CP_PATH/inference-model"
    cd "$CP_PATH/inference-model" && git checkout fea-lag-transformer
fi

# Install dependencies
pip3 install -r "$CP_PATH/inference-model/requirements.txt"

echo "Setup completed successfully."
