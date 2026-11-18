import type { Metadata } from "next";
import Link from "next/link";

import { RoomForm } from "@/features/facility/components/room-form";
import { timeZoneNames } from "@/lib/zoned-time";

export const metadata: Metadata = { title: "Register a room" };

// The page holds no data of the contexts: the build renders it once. The
// list of time zones is the one that the server knows, so that the browser
// renders the same options.
export default function RegisterRoomPage() {
  return (
    <>
      <p>
        <Link href="/">All rooms</Link>
      </p>
      <h1>Register a room</h1>
      <RoomForm timeZones={timeZoneNames()} />
    </>
  );
}
