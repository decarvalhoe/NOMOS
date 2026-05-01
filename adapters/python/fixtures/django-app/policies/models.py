from django.db import models


class Policy(models.Model):
    policy_id = models.CharField(max_length=32, unique=True)
    holder_name = models.CharField(max_length=128)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        verbose_name_plural = "policies"
