import Link from "next/link";

export default function ReservationNotFound() {
  return (
    <>
      <h1>Reservation not found</h1>
      <p>No reservation has this address. The link may be wrong, or incomplete.</p>
      <p>
        <Link href="/">See all rooms</Link>
      </p>
    </>
  );
}
