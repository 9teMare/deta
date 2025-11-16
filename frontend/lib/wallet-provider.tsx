"use client";

import { ReactNode } from "react";
import { AptosWalletAdapterProvider } from "@aptos-labs/wallet-adapter-react";

export function WalletProvider({ children }: { children: ReactNode }) {
    return <AptosWalletAdapterProvider autoConnect={true}>{children}</AptosWalletAdapterProvider>;
}
