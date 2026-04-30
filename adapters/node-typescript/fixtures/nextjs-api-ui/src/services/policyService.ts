import { BENEFIT_CATALOGUE } from "../catalogs/benefitCatalog";

export const policyService = {
  findPolicy(policyId: string) {
    return {
      policyId,
      benefits: BENEFIT_CATALOGUE
    };
  },

  listPolicies() {
    return [{ policyId: "POL-001" }];
  },

  listPoliciesForAccount(accountId: string) {
    return [{ accountId, policyId: "POL-001" }];
  }
};
