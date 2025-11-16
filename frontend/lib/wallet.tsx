"use client";

import { createContext, useContext, ReactNode } from "react";

// Simple wallet context - will be enhanced with actual wallet adapter
interface WalletContextType {
    account: string | null;
    connected: boolean;
    connect: () => Promise<void>;
    disconnect: () => void;
    signMessage: (message: string) => Promise<string>;
}

const WalletContext = createContext<WalletContextType | undefined>(undefined);

export function WalletProvider({ children }: { children: ReactNode }) {
    // For now, we'll use a simple implementation
    // In production, integrate with @aptos-labs/wallet-adapter-react
    const value: WalletContextType = {
        account: null,
        connected: false,
        connect: async () => {
            // Placeholder - will be implemented with actual wallet
            throw new Error("Wallet connection not implemented");
        },
        disconnect: () => {},
        signMessage: async () => {
            throw new Error("Sign message not implemented");
        },
    };

    return <WalletContext.Provider value={value}>{children}</WalletContext.Provider>;
}

export const useWallet = () => {
    const context = useContext(WalletContext);
    if (!context) {
        throw new Error("useWallet must be used within WalletProvider");
    }
    return context;
};
