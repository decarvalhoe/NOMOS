from app.catalogs.benefit_catalog import BENEFIT_CATALOG


class PolicyService:
    def find_policy(self, policy_id: str) -> dict:
        return {
            "policy_id": policy_id,
            "benefits": BENEFIT_CATALOG,
        }

    def list_policies(self) -> list[dict]:
        return [{"policy_id": "POL-001"}]

    def list_policies_for_account(self, account_id: str) -> list[dict]:
        return [{"account_id": account_id, "policy_id": "POL-001"}]
