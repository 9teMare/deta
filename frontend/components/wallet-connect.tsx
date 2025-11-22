"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "sonner";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { Wallet, LogOut, Copy, Check } from "lucide-react";
import { motion } from "framer-motion";

export function WalletConnect() {
    const { connect, disconnect, account, connected, wallets, isLoading } = useWallet();
    const [connecting, setConnecting] = useState(false);
    const [copied, setCopied] = useState(false);

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

    const copyAddress = () => {
        if (account?.address) {
            navigator.clipboard.writeText(account.address.toString());
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
            toast.success("Address copied to clipboard");
        }
    };

    if (connected && account) {
        return (
            <Card className="bg-white/5 backdrop-blur-md border-white/10 shadow-xl w-full max-w-md">
                <CardHeader className="pb-4">
                    <CardTitle className="flex items-center gap-2 text-white">
                        <div className="p-2 bg-green-500/20 rounded-full">
                            <Wallet className="w-5 h-5 text-green-400" />
                        </div>
                        Wallet Connected
                    </CardTitle>
                    <CardDescription className="text-gray-400">Your Aptos wallet is active</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="bg-black/20 p-4 rounded-xl border border-white/5 group relative overflow-hidden">
                        <div className="absolute inset-0 bg-gradient-to-r from-blue-500/10 to-purple-500/10 opacity-0 group-hover:opacity-100 transition-opacity" />
                        <p className="text-xs text-gray-400 mb-1 uppercase tracking-wider font-semibold">Account Address</p>
                        <div className="flex items-center justify-between gap-2">
                            <p className="text-sm font-mono text-white truncate">
                                {account.address.toString().slice(0, 6)}...{account.address.toString().slice(-6)}
                            </p>
                            <Button 
                                size="icon" 
                                variant="ghost" 
                                className="h-8 w-8 text-gray-400 hover:text-white hover:bg-white/10"
                                onClick={copyAddress}
                            >
                                {copied ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4" />}
                            </Button>
                        </div>
                    </div>
                    <Button 
                        onClick={handleDisconnect} 
                        variant="destructive" 
                        className="w-full bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20"
                    >
                        <LogOut className="w-4 h-4 mr-2" />
                        Disconnect Wallet
                    </Button>
                </CardContent>
            </Card>
        );
    }

    return (
        <Card className="bg-white/5 backdrop-blur-md border-white/10 shadow-xl w-full max-w-md overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-b from-blue-500/5 to-purple-500/5 pointer-events-none" />
            <CardHeader>
                <CardTitle className="text-white text-center text-2xl">Connect Wallet</CardTitle>
                <CardDescription className="text-gray-400 text-center">Choose your preferred wallet to continue</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 relative z-10">
                {wallets.length === 0 ? (
                    <div className="text-center py-8 bg-white/5 rounded-xl border border-white/5 border-dashed">
                        <p className="text-sm text-gray-400 mb-3">No Aptos wallets detected</p>
                        <p className="text-xs text-gray-500">
                            Please install{" "}
                            <a href="https://petra.app/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300 hover:underline font-medium">
                                Petra
                            </a>{" "}
                            or{" "}
                            <a href="https://pontem.network/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300 hover:underline font-medium">
                                Pontem
                            </a>
                        </p>
                    </div>
                ) : (
                    <div className="space-y-3">
                        {wallets.map((wallet, idx) => (
                            <motion.div
                                key={wallet.name}
                                initial={{ opacity: 0, x: -20 }}
                                animate={{ opacity: 1, x: 0 }}
                                transition={{ delay: idx * 0.1 }}
                            >
                                <Button
                                    onClick={() => handleConnect(wallet.name)}
                                    disabled={connecting || isLoading}
                                    className="w-full justify-between bg-white/5 hover:bg-white/10 border border-white/10 text-white h-14 px-4 group"
                                    variant="outline"
                                >
                                    <span className="flex items-center gap-3">
                                        {/* eslint-disable-next-line @next/next/no-img-element */}
                                        {wallet.icon && <img src={wallet.icon} alt={wallet.name} className="w-8 h-8 rounded-lg" />}
                                        <span className="font-medium text-lg">{wallet.name}</span>
                                    </span>
                                    <span className="text-xs text-gray-500 group-hover:text-blue-400 transition-colors">
                                        {connecting ? "Connecting..." : "Connect"}
                                    </span>
                                </Button>
                            </motion.div>
                        ))}
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
