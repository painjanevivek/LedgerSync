import { NextRequest, NextResponse } from "next/server";

import { addSecurityHeaders } from "@/lib/security";

export function proxy(_request: NextRequest) {
	void _request;
  return addSecurityHeaders(NextResponse.next());
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
