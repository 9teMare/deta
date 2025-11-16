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
            console.log("Transaction type:", typeof transaction);
            console.log("Transaction keys:", transaction ? Object.keys(transaction) : "undefined");
            console.log("Transaction.function:", transaction?.function);
            console.log("Transaction.data:", transaction?.data);
            console.log("signAndSubmitTransaction type:", typeof signAndSubmitTransaction);
            console.log("Full transaction object:", JSON.stringify(transaction, null, 2));

            if (!transaction) {
                throw new Error("Transaction is undefined");
            }

            // Sign and submit transaction
            // The wallet adapter expects InputTransactionData format
            // Try wrapping in 'data' if the direct format doesn't work
            if (!signAndSubmitTransaction) {
                throw new Error("signAndSubmitTransaction is not available");
            }

            // Try the transaction directly first
            // If that fails, the wallet adapter might need it wrapped differently
            let response;
            try {
                response = await signAndSubmitTransaction(transaction);
            } catch (error: any) {
                // If direct format fails, try wrapping in 'data' property
                console.log("Direct format failed, trying with 'data' wrapper");
                response = await signAndSubmitTransaction({
                    data: transaction,
                } as any);
            }

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
        <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
            <div className="container mx-auto px-4 py-8">
                <div className="max-w-6xl mx-auto">
                    {/* Header */}
                    <div className="text-center mb-8">
                        <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-2">DataX</h1>
                        <p className="text-lg text-gray-600 dark:text-gray-300">Decentralized Data Network on Aptos</p>
                        <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">Own your data. Control your access. Earn rewards.</p>
                    </div>

                    {/* Wallet Connection */}
                    <div className="mb-6">
                        <WalletConnect />
                    </div>

                    {connected && account && (
                        <>
                            {/* Initialize User */}
                            {checkingInit ? (
                                <Card className="mb-6">
                                    <CardContent className="pt-6">
                                        <p className="text-center text-muted-foreground">Checking initialization status...</p>
                                    </CardContent>
                                </Card>
                            ) : isInitialized ? (
                                <Card className="mb-6">
                                    <CardHeader>
                                        <CardTitle>Account Status</CardTitle>
                                        <CardDescription>Your account is initialized and ready to use</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
                                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                            </svg>
                                            <span className="font-medium">Account Initialized</span>
                                        </div>
                                    </CardContent>
                                </Card>
                            ) : (
                                <Card className="mb-6">
                                    <CardHeader>
                                        <CardTitle>Initialize Account</CardTitle>
                                        <CardDescription>Initialize your data store and vault (one-time setup)</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        <Button onClick={handleInitialize} className="w-full">
                                            Initialize User Account
                                        </Button>
                                        <p className="text-xs text-muted-foreground mt-2">
                                            This will create your DataStore and Vault resources on-chain
                                        </p>
                                    </CardContent>
                                </Card>
                            )}

                            {/* Main Tabs */}
                            <Tabs defaultValue="data" className="space-y-4">
                                <div className="flex justify-between items-center">
                                    <TabsList className="grid w-full grid-cols-3">
                                        <TabsTrigger value="data">Data Operations</TabsTrigger>
                                        <TabsTrigger value="access">Access Control</TabsTrigger>
                                        <TabsTrigger value="vault">My Vault</TabsTrigger>
                                    </TabsList>
                                    <Link href="/marketplace" className="ml-4">
                                        <Button variant="outline">Marketplace</Button>
                                    </Link>
                                </div>

                                <TabsContent value="data" className="space-y-4">
                                    <DataOperations account={accountAddress} />
                                </TabsContent>

                                <TabsContent value="access" className="space-y-4">
                                    <AccessControl account={accountAddress} />
                                </TabsContent>

                                <TabsContent value="vault" className="space-y-4">
                                    <VaultView account={accountAddress} />
                                </TabsContent>
                            </Tabs>
                        </>
                    )}

                    {!connected && (
                        <Card>
                            <CardContent className="pt-6">
                                <p className="text-center text-muted-foreground">Connect your wallet to get started</p>
                            </CardContent>
                        </Card>
                    )}
                </div>
            </div>
        </div>
    );
}
