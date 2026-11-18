import { Suspense } from "react";

import { RoomList } from "@/features/facility/components/room-list";

export default function RoomsPage() {
  return (
    <>
      <h1>Rooms</h1>
      <Suspense fallback={<p>Loading the rooms…</p>}>
        <RoomList />
      </Suspense>
    </>
  );
}
