import { Router } from "express";

import { policyService } from "../services/policyService";

export const accountRoutes = Router();

accountRoutes.get("/accounts/:accountId/policies", (request, response) => {
  response.json(policyService.listPoliciesForAccount(request.params.accountId));
});
