"use client";

import { useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import type { ConsoleSession } from "@/features/accounts/types";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type ConsoleSessionContextValue = Readonly<{
  session: ConsoleSession | null;
  sessionLoading: boolean;
  sessionError: string | null;
  online: boolean;
  publicOrigin: string;
  hasScope: (scope: string) => boolean;
  signOut: () => Promise<boolean>;
  signOutPending: boolean;
  signOutError: string | null;
}>;

const ConsoleSessionContext = createContext<ConsoleSessionContextValue | null>(null);

export function isConsoleSession(value: unknown): value is ConsoleSession {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<ConsoleSession>;
  return (
    typeof candidate.subject_id === "string" &&
    candidate.subject_id.length > 0 &&
    typeof candidate.tenant_id === "string" &&
    candidate.tenant_id.length > 0 &&
    typeof candidate.csrf_token === "string" &&
    candidate.csrf_token.length > 0 &&
    Array.isArray(candidate.scopes) &&
    candidate.scopes.every((scope) => typeof scope === "string") &&
    (candidate.environment === undefined ||
      candidate.environment === "local" ||
      candidate.environment === "production")
  );
}

export function ConsoleSessionBoundary({ children, initialSession, onResolved }: Readonly<{ children: ReactNode; initialSession?: ConsoleSession; onResolved?: (session: ConsoleSession) => void }>) {
  const router = useRouter();
  const [session, setSession] = useState<ConsoleSession | null>(initialSession ?? null);
  const [sessionLoading, setSessionLoading] = useState(!initialSession);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const [online, setOnline] = useState(true);
  const [publicOrigin, setPublicOrigin] = useState("http://127.0.0.1:3000");
  const [signOutPending, setSignOutPending] = useState(false);
  const [signOutError, setSignOutError] = useState<string | null>(null);
  const signOutInFlight = useRef(false);

  useEffect(() => {
    if (initialSession) return;
    let active = true;
    void (async () => {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (!active) return;
      if (response.ok && isConsoleSession(response.data)) {
        setSession(response.data);
        onResolved?.(response.data);
        setSessionError(null);
      } else {
        setSession(null);
        setSessionError(
          response.status === 401
            ? null
            : unavailableMessage(
                response.status,
                "the authorized session",
                response.requestReference,
              ),
        );
      }
      setSessionLoading(false);
    })();
    return () => {
      active = false;
    };
  }, [initialSession, onResolved]);

  useEffect(() => {
    const update = () => {
      setOnline(navigator.onLine);
      setPublicOrigin(window.location.origin);
    };
    update();
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);

  const hasScope = useCallback(
    (scope: string) => session?.scopes.includes(scope) ?? false,
    [session],
  );

  const signOut = useCallback(async () => {
    if (!session || signOutInFlight.current) return false;
    if (!online) {
      setSignOutError(
        "Sign-out could not be verified while the browser is offline. Reconnect and try again; the current workspace remains signed in.",
      );
      return false;
    }

    signOutInFlight.current = true;
    setSignOutPending(true);
    setSignOutError(null);
    try {
      const response = await fetch("/api/auth/sign-out", {
        method: "POST",
        headers: { "X-CSRF-Token": session.csrf_token },
      });
      if (response.ok || response.status === 401) {
        setSession(null);
        router.replace("/sign-in");
        router.refresh();
        return true;
      }

      const requestReference =
        response.headers.get("x-request-id") ?? "unavailable";
      setSignOutError(
        `Sign-out was not confirmed (${response.status}). The current workspace remains signed in. Request reference: ${requestReference}.`,
      );
      return false;
    } catch {
      setSignOutError(
        "Sign-out was not confirmed because the request could not be completed. The current workspace remains signed in; retry when connectivity is stable.",
      );
      return false;
    } finally {
      signOutInFlight.current = false;
      setSignOutPending(false);
    }
  }, [online, router, session]);

  const value = useMemo<ConsoleSessionContextValue>(
    () => ({
      session,
      sessionLoading,
      sessionError,
      online,
      publicOrigin,
      hasScope,
      signOut,
      signOutPending,
      signOutError,
    }),
    [
      hasScope,
      online,
      publicOrigin,
      session,
      sessionError,
      sessionLoading,
      signOut,
      signOutError,
      signOutPending,
    ],
  );

  return (
    <ConsoleSessionContext.Provider value={value}>
      {children}
    </ConsoleSessionContext.Provider>
  );
}

export function useConsoleSession() {
  const value = useContext(ConsoleSessionContext);
  if (!value) {
    throw new Error("useConsoleSession must be used within ConsoleSessionBoundary");
  }
  return value;
}
