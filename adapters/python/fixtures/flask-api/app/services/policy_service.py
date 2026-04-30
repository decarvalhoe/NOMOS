class PolicyService:
    def find_policy(self, policy_id: str) -> dict:
        return {"policy_id": policy_id, "status": "active"}

    def list_policies(self) -> list[dict]:
        return [{"policy_id": "POL-001", "status": "active"}]
