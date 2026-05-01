from rest_framework import serializers

from policies.models import Policy


class PolicySerializer(serializers.ModelSerializer):
    class Meta:
        model = Policy
        fields = ["policy_id", "holder_name", "created_at"]
