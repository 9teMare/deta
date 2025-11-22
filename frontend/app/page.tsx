"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { WalletConnect } from "@/components/wallet-connect";
import { DataOperations } from "@/components/data-operations";
import { AccessControl } from "@/components/access-control";
import { VaultView } from "@/components/vault-view";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { apiClient } from "@/lib/api";
import { buildTransaction } from "@/lib/aptos-client";
import { useState, useEffect } from "react";
import Link from "next/link";
import { AnimatedBackground } from "@/components/ui/animated-background";
import { motion } from "framer-motion";
import { Database, Shield, Lock, LayoutDashboard, ArrowRight } from "lucide-react";

export default function Home() {
    const { account, connected, signAndSubmitTransaction } = useWallet();
    const [isInitialized, setIsInitialized] = useState<boolean | null>(null);
    const [checkingInit, setCheckingInit] = useState(false);

    const handleInitialize = async () => {
        if (!connected || !account) {
            toast.error("Please connect your wallet first");
            return;
        }

        if (!signAndSubmitTransaction) {
            toast.error("Wallet does not support transaction signing");
            return;
        }

        try {
            // Build transaction to initialize user
            const transaction = await buildTransaction(
                {
                    moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab", // DataX module address
                    moduleName: "data_registry",
                    functionName: "init",
                    args: [],
                },
                account.address.toString()
            );

            console.log("Built transaction:", transaction);

            // Sign and submit transaction
            const response = await signAndSubmitTransaction(transaction);

            toast.success(`User initialized! Transaction: ${response.hash}`);

            // Refresh initialization status after successful initialization
            await checkInitializationStatus();
        } catch (error: any) {
            console.error("Initialize error:", error);
            toast.error(error.message || error.toString() || "Failed to initialize user");
        }
    };

    const checkInitializationStatus = async () => {
        if (!account) return;

        setCheckingInit(true);
        try {
            const result = await apiClient.checkInitialization(account.address.toString());
            setIsInitialized(result.initialized);
        } catch (error: any) {
            console.error("Failed to check initialization status:", error);
            // If check fails, assume not initialized
            setIsInitialized(false);
        } finally {
            setCheckingInit(false);
        }
    };

    // Check initialization status when account changes
    useEffect(() => {
        if (connected && account) {
            checkInitializationStatus();
        } else {
            setIsInitialized(null);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [connected, account]);

    const accountAddress = account?.address.toString() || "";

    return (
        <div className="min-h-screen relative">
            <AnimatedBackground />
            
            <div className="container mx-auto px-4 py-12 relative z-10">
                <div className="max-w-6xl mx-auto">
                    {/* Header */}
                    <motion.div 
                        initial={{ opacity: 0, y: -20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5 }}
                        className="text-center mb-12"
                    >
                        <h1 className="text-6xl font-extrabold text-transparent bg-clip-text bg-gradient-to-r from-blue-400 via-purple-500 to-indigo-600 mb-4 tracking-tight">
                            DataX
                        </h1>
                        <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
                            Decentralized Data Network on Aptos. Own your data. Control your access. Earn rewards.
                        </p>
                    </motion.div>

                    {/* Wallet Connection */}
                    <motion.div 
                        initial={{ opacity: 0, scale: 0.9 }}
                        animate={{ opacity: 1, scale: 1 }}
                        transition={{ duration: 0.5, delay: 0.2 }}
                        className="mb-10 flex justify-center"
                    >
                        <WalletConnect />
                    </motion.div>

                    {connected && account && (
                        <motion.div
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.5, delay: 0.4 }}
                        >
                            {/* Initialize User */}
                            {checkingInit ? (
                                <Card className="mb-8 border-white/10 bg-white/5 backdrop-blur-lg">
                                    <CardContent className="pt-6">
                                        <p className="text-center text-muted-foreground">Checking initialization status...</p>
                                    </CardContent>
                                </Card>
                            ) : isInitialized ? (
                                <Card className="mb-8 border-green-500/20 bg-green-500/5 backdrop-blur-lg">
                                    <CardHeader>
                                        <CardTitle className="flex items-center gap-2">
                                            <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                                            Account Status
                                        </CardTitle>
                                        <CardDescription>Your account is initialized and ready to use</CardDescription>
                                    </CardHeader>
                                </Card>
                            ) : (
                                <Card className="mb-8 border-yellow-500/20 bg-yellow-500/5 backdrop-blur-lg">
                                    <CardHeader>
                                        <CardTitle>Initialize Account</CardTitle>
                                        <CardDescription>Initialize your data store and vault (one-time setup)</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        <Button onClick={handleInitialize} className="w-full bg-gradient-to-r from-yellow-500 to-orange-500 hover:from-yellow-600 hover:to-orange-600 text-white border-0">
                                            Initialize User Account
                                        </Button>
                                        <p className="text-xs text-muted-foreground mt-2 text-center">
                                            This will create your DataStore and Vault resources on-chain
                                        </p>
                                    </CardContent>
                                </Card>
                            )}

                            {/* Main Tabs */}
                            <Tabs defaultValue="data" className="space-y-8">
                                <div className="flex flex-col md:flex-row justify-between items-center gap-4">
                                    <TabsList className="grid w-full md:w-auto grid-cols-3 bg-white/5 backdrop-blur-md border border-white/10 p-1 h-auto rounded-xl">
                                        <TabsTrigger value="data" className="data-[state=active]:bg-primary/20 data-[state=active]:text-primary py-3">
                                            <Database className="w-4 h-4 mr-2" />
                                            Data Operations
                                        </TabsTrigger>
                                        <TabsTrigger value="access" className="data-[state=active]:bg-primary/20 data-[state=active]:text-primary py-3">
                                            <Shield className="w-4 h-4 mr-2" />
                                            Access Control
                                        </TabsTrigger>
                                        <TabsTrigger value="vault" className="data-[state=active]:bg-primary/20 data-[state=active]:text-primary py-3">
                                            <Lock className="w-4 h-4 mr-2" />
                                            My Vault
                                        </TabsTrigger>
                                    </TabsList>
                                    <Link href="/marketplace">
                                        <Button variant="outline" className="w-full md:w-auto bg-white/5 backdrop-blur-md border-white/10 hover:bg-white/10">
                                            <LayoutDashboard className="w-4 h-4 mr-2" />
                                            Marketplace
                                            <ArrowRight className="w-4 h-4 ml-2" />
                                        </Button>
                                    </Link>
                                </div>

                                <TabsContent value="data" className="space-y-4 focus-visible:outline-none focus-visible:ring-0">
                                    <DataOperations account={accountAddress} />
                                </TabsContent>

                                <TabsContent value="access" className="space-y-4 focus-visible:outline-none focus-visible:ring-0">
                                    <AccessControl account={accountAddress} />
                                </TabsContent>

                                <TabsContent value="vault" className="space-y-4 focus-visible:outline-none focus-visible:ring-0">
                                    <VaultView account={accountAddress} />
                                </TabsContent>
                            </Tabs>
                        </motion.div>
                    )}

                    {!connected && (
                        <motion.div
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.5, delay: 0.4 }}
                        >
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-12">
                                <Card className="bg-white/5 backdrop-blur-sm border-white/10 hover:bg-white/10 transition-colors">
                                    <CardHeader>
                                        <Database className="w-10 h-10 text-blue-400 mb-2" />
                                        <CardTitle>Secure Storage</CardTitle>
                                        <CardDescription>Store your data hashes on the Aptos blockchain with full immutability.</CardDescription>
                                    </CardHeader>
                                </Card>
                                <Card className="bg-white/5 backdrop-blur-sm border-white/10 hover:bg-white/10 transition-colors">
                                    <CardHeader>
                                        <Shield className="w-10 h-10 text-purple-400 mb-2" />
                                        <CardTitle>Access Control</CardTitle>
                                        <CardDescription>Granular control over who can access your data and for how long.</CardDescription>
                                    </CardHeader>
                                </Card>
                                <Card className="bg-white/5 backdrop-blur-sm border-white/10 hover:bg-white/10 transition-colors">
                                    <CardHeader>
                                        <LayoutDashboard className="w-10 h-10 text-indigo-400 mb-2" />
                                        <CardTitle>Data Marketplace</CardTitle>
                                        <CardDescription>Monetize your datasets by listing them on the decentralized marketplace.</CardDescription>
                                    </CardHeader>
                                </Card>
                            </div>
                        </motion.div>
                    )}
                </div>
            </div>
        </div>
    );
}
