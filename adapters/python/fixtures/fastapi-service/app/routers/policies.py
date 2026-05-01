from fastapi import APIRouter, Depends

from app.schemas.policy import PolicyResponse
from app.services.policy_service import PolicyService

router = APIRouter(prefix="/policies", tags=["policies"])


@router.get("/", response_model=list[PolicyResponse])
def list_policies(service: PolicyService = Depends()):
    return service.list_policies()


@router.get("/{policy_id}", response_model=PolicyResponse)
def get_policy(policy_id: str, service: PolicyService = Depends()):
    return service.find_policy(policy_id)
