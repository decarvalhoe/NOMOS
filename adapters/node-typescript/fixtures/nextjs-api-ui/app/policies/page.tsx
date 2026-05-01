import { policyService } from "../../src/services/policyService";

export default function PoliciesPage() {
  const policies = policyService.listPolicies();
  return <main>{policies.length}</main>;
}
