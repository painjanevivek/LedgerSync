"use client";

import { useEffect, useState } from "react";

function relativeTime(value: string, now: number): string {
  const time = Date.parse(value);
  if (!Number.isFinite(time)) return "Time unavailable";
  const seconds = Math.max(0, Math.floor((now - time) / 1000));
  if (seconds < 60) return "Updated just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `Updated ${minutes} ${minutes === 1 ? "minute" : "minutes"} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `Updated ${hours} ${hours === 1 ? "hour" : "hours"} ago`;
  const days = Math.floor(hours / 24);
  return `Updated ${days} ${days === 1 ? "day" : "days"} ago`;
}

export function RelativeTime({ value, now: suppliedNow }: Readonly<{ value: string; now?: number }>) {
  const [clientNow, setClientNow] = useState(0);
  useEffect(() => {
    const timer = window.setTimeout(() => setClientNow(Date.now()), 0);
    return () => window.clearTimeout(timer);
  }, [value]);
  const now = suppliedNow ?? clientNow;
  if (now === 0) return <time dateTime={value} title={value}>Last verified time available</time>;
  return <time dateTime={value} title={value}>{relativeTime(value, now)}</time>;
}
