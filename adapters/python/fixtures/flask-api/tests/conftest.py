import pytest
from unittest.mock import MagicMock

from app import create_app


@pytest.fixture
def app():
    return create_app()


@pytest.fixture
def client(app):
    return app.test_client()


@pytest.fixture
def mock_policy_service(mocker):
    mock = MagicMock()
    mocker.patch("app.routes.policies.service", mock)
    return mock
