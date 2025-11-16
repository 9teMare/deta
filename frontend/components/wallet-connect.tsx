"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "sonner";
import { useWallet } from "@aptos-labs/wallet-adapter-react";

export function WalletConnect() {
    const { connect, disconnect, account, connected, wallets, isLoading } = useWallet();

    const [connecting, setConnecting] = useState(false);

    const handleConnect = async (walletName: string) => {
        try {
            setConnecting(true);
            await connect(walletName);
            toast.success("Wallet connected successfully");
        } catch (error: any) {
            toast.error(error.message || "Failed to connect wallet");
        } finally {
            setConnecting(false);
        }
    };

    const handleDisconnect = async () => {
        try {
            await disconnect();
            toast.success("Wallet disconnected successfully");
        } catch (error: any) {
            toast.error(error.message || "Failed to disconnect wallet");
        }
    };

    if (connected && account) {
        return (
            <Card>
                <CardHeader>
                    <CardTitle>Wallet Connected</CardTitle>
                    <CardDescription>Your Aptos wallet is connected</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <p className="text-sm font-medium mb-2">Account Address</p>
                        <p className="text-sm font-mono bg-muted p-2 rounded break-all">{account.address.toString()}</p>
                    </div>
                    <Button onClick={handleDisconnect} variant="destructive" className="w-full">
                        Disconnect Wallet
                    </Button>
                </CardContent>
            </Card>
        );
    }

    return (
        <Card>
            <CardHeader>
                <CardTitle>Connect Wallet</CardTitle>
                <CardDescription>Connect your Aptos wallet to get started</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {wallets.length === 0 ? (
                    <div className="text-center py-4">
                        <p className="text-sm text-muted-foreground mb-2">No Aptos wallets detected</p>
                        <p className="text-xs text-muted-foreground">
                            Please install an Aptos wallet extension like{" "}
                            <a href="https://petra.app/" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">
                                Petra
                            </a>{" "}
                            or{" "}
                            <a href="https://pontem.network/" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">
                                Pontem
                            </a>
                        </p>
                    </div>
                ) : (
                    <div className="space-y-2">
                        {wallets.map((wallet) => (
                            <Button
                                key={wallet.name}
                                onClick={() => handleConnect(wallet.name)}
                                disabled={connecting || isLoading}
                                className="w-full justify-start"
                                variant="outline"
                            >
                                {wallet.icon && <img src={wallet.icon} alt={wallet.name} className="w-5 h-5 mr-2" />}
                                {connecting ? "Connecting..." : `Connect ${wallet.name}`}
                            </Button>
                        ))}
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
