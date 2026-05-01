from flask import Blueprint, jsonify

from app.services.policy_service import PolicyService

policies_bp = Blueprint("policies", __name__)
service = PolicyService()


@policies_bp.route("/policies", methods=["GET"])
def list_policies():
    return jsonify(service.list_policies())


@policies_bp.route("/policies/<policy_id>", methods=["GET"])
def get_policy(policy_id):
    return jsonify(service.find_policy(policy_id))
