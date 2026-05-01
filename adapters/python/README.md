# Python Adapter

This adapter profile documents the Python conventions that the Nomos detector
treats as auditable product signals.

## Frameworks Covered

- **FastAPI** — decorator-based routes (`@app.get`, `@router.post`), Pydantic
  models as serializers, dependency injection services.
- **Django** — `urlpatterns` in `urls.py`, class-based and function views,
  DRF serializers, management commands.
- **Flask** — `@app.route` and `@blueprint.route` decorators, service modules.

## Backend Conventions

- API routes: FastAPI router decorators (`@router.get`, `@router.post`), Flask
  `@app.route` / `@blueprint.route`, Django `urlpatterns` with `path()` and
  `re_path()`.
- Services: modules under `services/`, files ending in `_service.py` or
  `service.py`, and classes/functions with `Service` in their name.
- Serializers: Pydantic `BaseModel` subclasses, DRF `Serializer` and
  `ModelSerializer` subclasses, marshmallow `Schema` subclasses.
- Mocks: `unittest.mock.patch`, `pytest-mock` fixtures (`mocker`), files under
  `mocks/` or `__mocks__/`, `conftest.py` fixtures with `mock` in their name.
- Fixtures: `conftest.py` pytest fixtures, `fixtures/` directories, JSON/YAML
  fixture files, Django `fixtures/*.json`.
- Hardcoded catalogues: Python constants (module-level `list`/`dict` literals)
  that expose catalogues, plans, statuses, tiers, products, options, or similar
  business enumerations without an upstream canonical source.

## Official Fixtures

- `fixtures/fastapi-service` — FastAPI app with routes, services, Pydantic
  serializers, mocks, test fixtures, and a hardcoded catalogue.
- `fixtures/django-app` — Django project with URL patterns, views, DRF
  serializers, and Django fixtures.
- `fixtures/flask-api` — Flask app with route decorators, services, and mocks.
