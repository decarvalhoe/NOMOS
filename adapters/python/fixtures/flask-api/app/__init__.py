from flask import Flask


def create_app():
    app = Flask(__name__)

    from app.routes import policies_bp

    app.register_blueprint(policies_bp, url_prefix="/api/v1")
    return app
