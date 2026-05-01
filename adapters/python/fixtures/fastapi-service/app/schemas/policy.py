from pydantic import BaseModel


class BenefitSchema(BaseModel):
    code: str
    label: str
    annual_limit: int


class PolicyResponse(BaseModel):
    policy_id: str
    benefits: list[BenefitSchema] = []
