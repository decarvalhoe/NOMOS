from django.urls import path

from policies import views

urlpatterns = [
    path("policies/", views.PolicyListView.as_view(), name="policy-list"),
    path("policies/<str:pk>/", views.PolicyDetailView.as_view(), name="policy-detail"),
]
