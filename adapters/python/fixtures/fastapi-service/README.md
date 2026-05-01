# FastAPI Service Fixture

Compatibility fixture for the Python adapter (NOM-403). Demonstrates:

- FastAPI router-based route detection (`@router.get`, `@router.post`)
- Pydantic serializer detection (`BaseModel` subclasses)
- Service class detection (`PolicyService`)
- Mock detection (`pytest-mock` / `mocker.patch`)
- Fixture detection (`conftest.py` fixtures, `fixtures/` directory)
- Hardcoded catalogue detection (`BENEFIT_CATALOG`)
