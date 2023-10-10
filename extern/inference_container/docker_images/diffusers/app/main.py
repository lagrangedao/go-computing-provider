import asyncio
import functools
import logging
import os
from typing import Dict, Type

from api_inference_community.routes import pipeline_route, status_ok
from app import idle
from app.pipelines import ImageToImagePipeline, Pipeline, TextToImagePipeline
from starlette.applications import Starlette
from starlette.middleware import Middleware
from starlette.middleware.gzip import GZipMiddleware
from starlette.routing import Route
from starlette.templating import Jinja2Templates
from starlette.routing import Mount
from starlette.staticfiles import StaticFiles

TASK = os.getenv("TASK")
MODEL_ID = os.getenv("MODEL_ID")

# Add the allowed tasks
# Supported tasks are:
# - text-generation
# - text-classification
# - token-classification
# - translation
# - summarization
# - automatic-speech-recognition
# - ...
# For instance
# from app.pipelines import AutomaticSpeechRecognitionPipeline
# ALLOWED_TASKS = {"automatic-speech-recognition": AutomaticSpeechRecognitionPipeline}
# You can check the requirements and expectations of each pipelines in their respective
# directories. Implement directly within the directories.
ALLOWED_TASKS: Dict[str, Type[Pipeline]] = {
    "text-to-image": TextToImagePipeline,
    "image-to-image": ImageToImagePipeline,
}


@functools.lru_cache()
def get_pipeline() -> Pipeline:
    task = os.environ["TASK"]
    model_id = os.environ["MODEL_ID"]
    if task not in ALLOWED_TASKS:
        raise EnvironmentError(f"{task} is not a valid pipeline for model : {model_id}")
    return ALLOWED_TASKS[task](model_id)


templates = Jinja2Templates(directory='app/templates')


async def homepage(request):
    api_url = 'https://' + os.environ['result_url']
    python_code = f"""
    import requests
    
    API_URL = {api_url}

    def query(payload):
      response = requests.post(API_URL, json=payload)
      return response.content
    image_bytes = query({{
      "inputs": "Astronaut riding a horse",
    }})
    # You can access the image with PIL.Image for example
    import io
    from PIL import Image
    image = Image.open(io.BytesIO(image_bytes))
    """

    curl_code = f"""
    curl  {api_url} \\
        -X POST \\
        -d '{{"inputs": "Astronaut riding a horse"}}' \\
        --output horse.jpg
    """

    javaScript_code = f"""
    async function query(data) {{
        const response = await fetch(
            " {api_url}",
            {{
                method: "POST",
                body: JSON.stringify(data),
            }}
        );
        const result = await response.blob();
        return result;
        }}
    query({{"inputs": "Astronaut riding a horse"}}).then((response) => {{
        // Use image
    }});
    """

    context = {'request': request, 'api_url': api_url, 'python_code': python_code,
               'javaScript_code': javaScript_code, 'curl_code': curl_code}
    return templates.TemplateResponse('index.html', context)


routes = [
    Mount('/static', app=StaticFiles(directory="app/templates/static"), name="static"),

    Route('/', endpoint=homepage),
    Route("/{whatever:path}", status_ok),
    Route("/{whatever:path}", pipeline_route, methods=["POST"]),
]

middleware = [Middleware(GZipMiddleware, minimum_size=1000)]
if os.environ.get("DEBUG", "") == "1":
    from starlette.middleware.cors import CORSMiddleware

    middleware.append(
        Middleware(
            CORSMiddleware,
            allow_origins=["*"],
            allow_headers=["*"],
            allow_methods=["*"],
        )
    )

app = Starlette(routes=routes, middleware=middleware)


@app.on_event("startup")
async def startup_event():
    reset_logging()
    # Link between `api-inference-community` and framework code.
    if idle.UNLOAD_IDLE:
        asyncio.create_task(idle.live_check_loop(), name="live_check_loop")
    app.get_pipeline = get_pipeline
    try:
        get_pipeline()
    except Exception:
        # We can fail so we can show exception later.
        pass


def reset_logging():
    logging.basicConfig(
        level=logging.DEBUG,
        format="%(asctime)s - %(levelname)s - %(message)s",
        force=True,
    )


if __name__ == "__main__":
    reset_logging()
    try:
        get_pipeline()
    except Exception:
        # We can fail so we can show exception later.
        pass
