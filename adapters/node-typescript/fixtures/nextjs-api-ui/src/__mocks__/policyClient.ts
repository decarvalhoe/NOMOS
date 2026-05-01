import { http, HttpResponse } from "msw";

export const policyClientHandlers = [
  http.get("/api/policies", () => {
    return HttpResponse.json([{ policyId: "POL-MOCK" }]);
  })
];
