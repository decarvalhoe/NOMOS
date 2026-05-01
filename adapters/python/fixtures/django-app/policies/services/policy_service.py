from policies.models import Policy


class PolicyService:
    def get_active_policies(self):
        return Policy.objects.filter(is_active=True)

    def cancel_policy(self, policy_id: str) -> bool:
        policy = Policy.objects.get(policy_id=policy_id)
        policy.is_active = False
        policy.save()
        return True
