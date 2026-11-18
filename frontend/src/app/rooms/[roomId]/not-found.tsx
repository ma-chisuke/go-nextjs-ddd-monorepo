import Link from "next/link";

export default function RoomNotFound() {
  return (
    <>
      <h1>Room not found</h1>
      <p>No room has this address. The link may be wrong, or incomplete.</p>
      <p>
        <Link href="/">See all rooms</Link>
      </p>
    </>
  );
}
