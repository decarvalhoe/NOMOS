import { NextRequest, NextResponse } from "next/server";

import { policyService } from "../../../src/services/policyService";

export async function GET(request: NextRequest) {
  const policyId = request.nextUrl.searchParams.get("policyId") ?? "demo";
  return NextResponse.json(policyService.findPolicy(policyId));
}
