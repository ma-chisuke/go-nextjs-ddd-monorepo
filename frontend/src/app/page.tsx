import Link from "next/link";
import { Suspense } from "react";

import { RoomList } from "@/features/facility/components/room-list";

export default function RoomsPage() {
  return (
    <>
      <h1>Rooms</h1>
      <p>
        <Link href="/rooms/new">Register a room</Link>
      </p>
      <Suspense fallback={<p>Loading the rooms…</p>}>
        <RoomList />
      </Suspense>
    </>
  );
}
