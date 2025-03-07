#!/usr/bin/env python3
import os
import sys
import subprocess

def main():
    # Get current directory
    current_dir = os.path.dirname(os.path.abspath(__file__))
    
    # Path to proto file
    proto_file = os.path.join(current_dir, "scripts", "grpc", "transcribe.proto")
    
    # Check if proto file exists
    if not os.path.exists(proto_file):
        print(f"Error: Proto file {proto_file} not found")
        return 1
    
    # Generate gRPC Python code
    cmd = [
        sys.executable, "-m", "grpc_tools.protoc",
        f"-I{os.path.join(current_dir, 'scripts', 'grpc')}",
        f"--python_out={os.path.join(current_dir, 'scripts', 'grpc')}",
        f"--grpc_python_out={os.path.join(current_dir, 'scripts', 'grpc')}",
        proto_file
    ]
    
    try:
        print(f"Running command: {' '.join(cmd)}")
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        print(result.stdout)
        print("gRPC Python code generated successfully")
        return 0
    except subprocess.CalledProcessError as e:
        print(f"Error generating gRPC Python code: {e}")
        print(f"stdout: {e.stdout}")
        print(f"stderr: {e.stderr}")
        return 1

if __name__ == "__main__":
    sys.exit(main())