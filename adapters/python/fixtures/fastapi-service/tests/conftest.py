import pytest


@pytest.fixture
def mock_policy_service(mocker):
    return mocker.patch(
        "app.services.policy_service.PolicyService",
        autospec=True,
    )


@pytest.fixture
def sample_policy():
    return {"policy_id": "POL-TEST", "benefits": []}
