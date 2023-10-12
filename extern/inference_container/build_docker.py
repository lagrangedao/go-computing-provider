#!/usr/bin/env python
import argparse
import os
import subprocess
import sys
import uuid


def run(command, output_file=None):
    print(" ".join(command))
    try:
        if output_file:
            with open(output_file, "w") as f:
                p = subprocess.run(command, stdout=f, stderr=subprocess.STDOUT, check=True)
        else:
            p = subprocess.run(command, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error: {e}")
        sys.exit(e.returncode)


def build(root_dir: str, framework: str, tag: str, build_log: str, is_gpu: bool):
    if tag is None:
        DEFAULT_HOSTNAME = os.getenv("DEFAULT_HOSTNAME")
        hostname = DEFAULT_HOSTNAME
        tag_id = str(uuid.uuid4())[:5]
        tag = f"{framework}-{tag_id}"
        container_tag = f"{hostname}/api-inference/community:{tag}"
    else:
        container_tag = tag

    command = ["docker", "build", f"{root_dir}/docker_images/{framework}", "-t", container_tag]
    run(command, build_log)
    return tag


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "root_dir",
        type=str,
        help="python file root dir",
    )
    parser.add_argument(
        "framework",
        type=str,
        help="Which framework image to build.",
    )
    parser.add_argument(
        "tag",
        type=str,
        help="Image new tag",
    )
    parser.add_argument(
        "build_log",
        type=str,
        help="build log file name",
    )
    parser.add_argument(
        "--out",
        type=str,
        help="Where to store the new tags",
    )
    parser.add_argument(
        "--gpu",
        action="store_true",
        help="Build the GPU version of the model",
    )
    args = parser.parse_args()

    tag = build(args.root_dir, args.framework, args.tag, args.build_log, args.gpu)
    print(tag)


if __name__ == "__main__":
    main()
