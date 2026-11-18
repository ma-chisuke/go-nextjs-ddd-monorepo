import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";

import "./globals.css";

export const metadata: Metadata = {
  title: { default: "Meeting rooms", template: "%s · Meeting rooms" },
  description: "Book a meeting room: the sample of go-nextjs-ddd-monorepo.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="site-header">
          <Link href="/" className="site-name">
            Meeting rooms
          </Link>
        </header>
        <main>{children}</main>
      </body>
    </html>
  );
}
