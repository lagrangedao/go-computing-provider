import json
import argparse
from huggingface_hub import HfApi


def get_model_info(args) -> [str, str, str]:
    try:
        model_id = args.model_id
        info = HfApi().model_info(model_id)
    except Exception as e:
        raise ValueError(
            f"The hub has no information on {model_id}, does it exist: {e}"
        )
    try:
        task = info.pipeline_tag
    except Exception:
        raise ValueError(
            f"The hub has no `pipeline_tag` on {model_id}, you can set it in the `README.md` yaml header"
        )
    try:
        framework = info.library_name
    except Exception:
        raise ValueError(
            f"The hub has no `library_name` on {model_id}, you can set it in the `README.md` yaml header"
        )

    model = {'model_id': model_id, 'task': task, 'framework': framework.replace("-", "_")}
    print(json.dumps(model))


def main():
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers()
    parser_get = subparsers.add_parser(
        "model_info", help="Get the model of task type and framework"
    )
    parser_get.add_argument(
        "model_id",
        type=str,
        help="Which model_id to check.",
    )
    parser_get.set_defaults(func=get_model_info)
    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
