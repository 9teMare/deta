"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { apiClient } from "@/lib/api";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { buildTransaction } from "@/lib/aptos-client";
import { Shield, UserCheck, UserX, Search, Clock, RefreshCw, Key } from "lucide-react";
import { motion } from "framer-motion";

interface AccessControlProps {
    account: string;
}

interface AccessRequest {
    dataset_id: number;
    requester: string;
    message?: string;
    requested_at?: string;
}

export function AccessControl({ account }: AccessControlProps) {
    const { signAndSubmitTransaction } = useWallet();
    const [datasetId, setDatasetId] = useState("");
    const [requester, setRequester] = useState("");
    const [expiresAt, setExpiresAt] = useState("");
    const [checkOwner, setCheckOwner] = useState("");
    const [checkDatasetId, setCheckDatasetId] = useState("");
    const [checkRequester, setCheckRequester] = useState("");
    const [loading, setLoading] = useState(false);
    const [accessResult, setAccessResult] = useState<boolean | null>(null);
    const [accessRequests, setAccessRequests] = useState<AccessRequest[]>([]);
    const [loadingRequests, setLoadingRequests] = useState(false);

    const loadAccessRequests = async () => {
        setLoadingRequests(true);
        try {
            const requests = await apiClient.getAccessRequests(account);
            setAccessRequests(requests as AccessRequest[]);
        } catch (error: any) {
            console.error("Failed to load access requests:", error);
        } finally {
            setLoadingRequests(false);
        }
    };

    useEffect(() => {
        if (account) {
            loadAccessRequests();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [account]);

    const handleGrantAccess = async (reqDatasetId?: number, reqRequester?: string) => {
        const finalDatasetId = reqDatasetId || parseInt(datasetId);
        const finalRequester = reqRequester || requester;

        if (!finalDatasetId || !finalRequester || (!reqDatasetId && !expiresAt)) {
            toast.error("Please fill in all fields");
            return;
        }

        if (!signAndSubmitTransaction) {
            toast.error("Wallet does not support transaction signing");
            return;
        }

        setLoading(true);
        try {
            // Convert expiresAt to Unix timestamp
            let expiresAtTimestamp: number;
            if (reqDatasetId) {
                // Default to 1 year from now for requests
                expiresAtTimestamp = Math.floor(Date.now() / 1000) + 365 * 24 * 60 * 60;
            } else {
                const date = new Date(expiresAt);
                expiresAtTimestamp = Math.floor(date.getTime() / 1000);
            }

            const transaction = await buildTransaction(
                {
                    moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab",
                    moduleName: "AccessControl",
                    functionName: "grant_access",
                    args: [finalDatasetId, finalRequester, expiresAtTimestamp],
                },
                account
            );

            const response = await signAndSubmitTransaction(transaction);
            toast.success(`Access granted! Transaction: ${response.hash}`);

            // Refresh requests
            await loadAccessRequests();
            setDatasetId("");
            setRequester("");
            setExpiresAt("");
        } catch (error: any) {
            toast.error(error.message || "Failed to grant access");
        } finally {
            setLoading(false);
        }
    };

    const handleRevokeAccess = async () => {
        if (!datasetId || !requester) {
            toast.error("Please fill in all fields");
            return;
        }

        if (!signAndSubmitTransaction) {
            toast.error("Wallet does not support transaction signing");
            return;
        }

        setLoading(true);
        try {
            const transaction = await buildTransaction(
                {
                    moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab",
                    moduleName: "AccessControl",
                    functionName: "revoke_access",
                    args: [parseInt(datasetId), requester],
                },
                account
            );

            const response = await signAndSubmitTransaction(transaction);
            toast.success(`Access revoked! Transaction: ${response.hash}`);

            setDatasetId("");
            setRequester("");
        } catch (error: any) {
            toast.error(error.message || "Failed to revoke access");
        } finally {
            setLoading(false);
        }
    };

    const handleCheckAccess = async () => {
        if (!checkOwner || !checkDatasetId || !checkRequester) {
            toast.error("Please fill in all fields");
            return;
        }

        setLoading(true);
        try {
            const result = await apiClient.checkAccess(checkOwner, parseInt(checkDatasetId), checkRequester);
            setAccessResult(result.has_access);
            toast.success(`Access check complete: ${result.has_access ? "Has access" : "No access"}`);
        } catch (error: any) {
            toast.error(error.message || "Failed to check access");
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <Card className="bg-white/5 backdrop-blur-md border-white/10">
                <CardHeader>
                    <div className="flex justify-between items-center">
                        <div>
                            <CardTitle className="flex items-center gap-2">
                                <Clock className="w-5 h-5 text-yellow-400" />
                                Pending Access Requests
                            </CardTitle>
                            <CardDescription>Review and grant access to pending requests</CardDescription>
                        </div>
                        <Button 
                            onClick={loadAccessRequests} 
                            disabled={loadingRequests} 
                            variant="outline" 
                            size="sm"
                            className="bg-white/5 border-white/10 hover:bg-white/10"
                        >
                            <RefreshCw className={`w-4 h-4 mr-2 ${loadingRequests ? "animate-spin" : ""}`} />
                            Refresh
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    {accessRequests.length === 0 ? (
                        <div className="text-center py-8 bg-black/20 rounded-xl border border-white/5 border-dashed">
                            <p className="text-sm text-gray-400">No pending access requests</p>
                        </div>
                    ) : (
                        <div className="space-y-3">
                            {accessRequests.map((request, idx) => (
                                <motion.div 
                                    key={idx}
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: idx * 0.1 }}
                                >
                                    <Card className="bg-black/20 border-white/5 overflow-hidden">
                                        <CardContent className="pt-4">
                                            <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                                                <div className="space-y-1">
                                                    <div className="flex items-center gap-2">
                                                        <span className="bg-blue-500/20 text-blue-300 text-xs px-2 py-0.5 rounded-full">
                                                            Dataset #{request.dataset_id}
                                                        </span>
                                                    </div>
                                                    <div className="text-sm text-gray-300 font-mono truncate max-w-xs">
                                                        <span className="text-gray-500">Requester:</span> {request.requester}
                                                    </div>
                                                    {request.message && (
                                                        <div className="text-sm text-gray-400 italic">&quot;{request.message}&quot;</div>
                                                    )}
                                                </div>
                                                <Button
                                                    size="sm"
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        handleGrantAccess(request.dataset_id, request.requester);
                                                    }}
                                                    disabled={loading}
                                                    className="bg-green-500/20 text-green-400 hover:bg-green-500/30 border border-green-500/20"
                                                >
                                                    <UserCheck className="w-4 h-4 mr-2" />
                                                    Grant Access
                                                </Button>
                                            </div>
                                        </CardContent>
                                    </Card>
                                </motion.div>
                            ))}
                        </div>
                    )}
                </CardContent>
            </Card>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <Card className="bg-white/5 backdrop-blur-md border-white/10">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Key className="w-5 h-5 text-green-400" />
                            Grant Access
                        </CardTitle>
                        <CardDescription>Grant access to a requester</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div>
                            <Label htmlFor="grantDatasetId" className="text-gray-300">Dataset ID</Label>
                            <Input
                                id="grantDatasetId"
                                type="number"
                                placeholder="Enter dataset ID"
                                value={datasetId}
                                onChange={(e) => setDatasetId(e.target.value)}
                                className="bg-black/20 border-white/10 mt-1"
                            />
                        </div>
                        <div>
                            <Label htmlFor="requester" className="text-gray-300">Requester Address</Label>
                            <Input
                                id="requester"
                                placeholder="0x..."
                                value={requester}
                                onChange={(e) => setRequester(e.target.value)}
                                className="bg-black/20 border-white/10 font-mono mt-1"
                            />
                        </div>
                        <div>
                            <Label htmlFor="expiresAt" className="text-gray-300">Expires At</Label>
                            <Input 
                                id="expiresAt" 
                                type="datetime-local" 
                                value={expiresAt} 
                                onChange={(e) => setExpiresAt(e.target.value)}
                                className="bg-black/20 border-white/10 mt-1" 
                            />
                        </div>
                        <Button
                            onClick={(e) => {
                                e.preventDefault();
                                handleGrantAccess();
                            }}
                            disabled={loading}
                            className="w-full bg-green-600 hover:bg-green-500 text-white"
                        >
                            {loading ? "Granting..." : "Grant Access"}
                        </Button>
                    </CardContent>
                </Card>

                <Card className="bg-white/5 backdrop-blur-md border-white/10">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <UserX className="w-5 h-5 text-red-400" />
                            Revoke Access
                        </CardTitle>
                        <CardDescription>Revoke access from a requester</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div>
                            <Label htmlFor="revokeDatasetId" className="text-gray-300">Dataset ID</Label>
                            <Input
                                id="revokeDatasetId"
                                type="number"
                                placeholder="Enter dataset ID"
                                value={datasetId}
                                onChange={(e) => setDatasetId(e.target.value)}
                                className="bg-black/20 border-white/10 mt-1"
                            />
                        </div>
                        <div>
                            <Label htmlFor="revokeRequester" className="text-gray-300">Requester Address</Label>
                            <Input
                                id="revokeRequester"
                                placeholder="0x..."
                                value={requester}
                                onChange={(e) => setRequester(e.target.value)}
                                className="bg-black/20 border-white/10 font-mono mt-1"
                            />
                        </div>
                        <div className="pt-[72px]"> {/* Spacer to align buttons */}
                            <Button 
                                onClick={handleRevokeAccess} 
                                disabled={loading} 
                                variant="destructive" 
                                className="w-full bg-red-500/20 hover:bg-red-500/30 text-red-400 border border-red-500/20"
                            >
                                {loading ? "Revoking..." : "Revoke Access"}
                            </Button>
                        </div>
                    </CardContent>
                </Card>
            </div>

            <Card className="bg-white/5 backdrop-blur-md border-white/10">
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Search className="w-5 h-5 text-blue-400" />
                        Check Access
                    </CardTitle>
                    <CardDescription>Check if a requester has access to a dataset</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                            <Label htmlFor="checkOwner" className="text-gray-300">Owner Address</Label>
                            <Input
                                id="checkOwner"
                                placeholder="0x..."
                                value={checkOwner}
                                onChange={(e) => setCheckOwner(e.target.value)}
                                className="bg-black/20 border-white/10 font-mono mt-1"
                            />
                        </div>
                        <div>
                            <Label htmlFor="checkDatasetId" className="text-gray-300">Dataset ID</Label>
                            <Input
                                id="checkDatasetId"
                                type="number"
                                placeholder="Enter dataset ID"
                                value={checkDatasetId}
                                onChange={(e) => setCheckDatasetId(e.target.value)}
                                className="bg-black/20 border-white/10 mt-1"
                            />
                        </div>
                        <div>
                            <Label htmlFor="checkRequester" className="text-gray-300">Requester Address</Label>
                            <Input
                                id="checkRequester"
                                placeholder="0x..."
                                value={checkRequester}
                                onChange={(e) => setCheckRequester(e.target.value)}
                                className="bg-black/20 border-white/10 font-mono mt-1"
                            />
                        </div>
                    </div>
                    
                    <Button onClick={handleCheckAccess} disabled={loading} className="w-full bg-blue-600 hover:bg-blue-500">
                        {loading ? "Checking..." : "Check Access"}
                    </Button>
                    
                    {accessResult !== null && (
                        <motion.div 
                            initial={{ opacity: 0, scale: 0.95 }}
                            animate={{ opacity: 1, scale: 1 }}
                            className={`p-4 rounded-lg border ${
                                accessResult 
                                    ? "bg-green-500/10 border-green-500/20 text-green-400" 
                                    : "bg-red-500/10 border-red-500/20 text-red-400"
                            } flex items-center justify-center gap-2 font-medium`}
                        >
                            {accessResult ? (
                                <><UserCheck className="w-5 h-5" /> Access Granted</>
                            ) : (
                                <><UserX className="w-5 h-5" /> Access Denied</>
                            )}
                        </motion.div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
