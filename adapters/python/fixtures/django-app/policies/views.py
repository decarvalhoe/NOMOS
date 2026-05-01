from rest_framework import generics

from policies.models import Policy
from policies.serializers import PolicySerializer


class PolicyListView(generics.ListAPIView):
    queryset = Policy.objects.all()
    serializer_class = PolicySerializer


class PolicyDetailView(generics.RetrieveAPIView):
    queryset = Policy.objects.all()
    serializer_class = PolicySerializer
