#!/usr/bin/env python
import argparse
import os
import subprocess
import sys
import uuid


def run(command):
    print(" ".join(command))
    p = subprocess.run(command)
    if p.returncode != 0:
        sys.exit(p.returncode)


def build(root_dir: str, framework: str, tag: str, is_gpu: bool):
    if tag is None:
        DEFAULT_HOSTNAME = os.getenv("DEFAULT_HOSTNAME")
        hostname = DEFAULT_HOSTNAME
        tag_id = str(uuid.uuid4())[:5]
        tag = f"{framework}-{tag_id}"
        container_tag = f"{hostname}/api-inference/community:{tag}"
    else:
        container_tag = tag

    command = ["docker", "build", f"{root_dir}/docker_images/{framework}", "-t", container_tag]
    run(command)

    # password = os.environ["REGISTRY_PASSWORD"]
    # username = os.environ["REGISTRY_USERNAME"]
    # command = ["echo", password]
    # ecr_login = subprocess.Popen(command, stdout=subprocess.PIPE)
    # docker_login = subprocess.Popen(
    #     ["docker", "login", "-u", username, "--password-stdin", hostname],
    #     stdin=ecr_login.stdout,
    #     stdout=subprocess.PIPE,
    # )
    # docker_login.communicate()
    #
    # command = ["docker", "push", container_tag]
    # run(command)
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

    tag = build(args.root_dir, args.framework, args.tag, args.gpu)
    print(tag)


if __name__ == "__main__":
    main()
