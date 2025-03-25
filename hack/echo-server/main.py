from flask import Flask, request
import json
from http import HTTPMethod
import logging

log = logging.getLogger('werkzeug')
log.setLevel(logging.ERROR)

app = Flask(__name__)

@app.route("/<path:path>", methods=[m for m in HTTPMethod])
def echo(path):
    print(
        json.dumps(
            dict(
                path=path,
                params=request.args.to_dict(),
                headers=dict(request.headers),
                method=request.method,
                body=request.get_json(silent=True),
            ),
            indent=4,
        ) + "\n" + "-" * 40 + "\n"
    )
    return ""
