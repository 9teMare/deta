"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { apiClient } from "@/lib/api";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { buildTransaction } from "@/lib/aptos-client";

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
        <div className="space-y-4">
            <Card>
                <CardHeader>
                    <div className="flex justify-between items-center">
                        <div>
                            <CardTitle>Pending Access Requests</CardTitle>
                            <CardDescription>Review and grant access to pending requests</CardDescription>
                        </div>
                        <Button onClick={loadAccessRequests} disabled={loadingRequests} variant="outline" size="sm">
                            {loadingRequests ? "Loading..." : "Refresh"}
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    {accessRequests.length === 0 ? (
                        <p className="text-sm text-muted-foreground text-center py-4">No pending access requests</p>
                    ) : (
                        <div className="space-y-3">
                            {accessRequests.map((request, idx) => (
                                <Card key={idx} className="border">
                                    <CardContent className="pt-4">
                                        <div className="flex justify-between items-start">
                                            <div className="space-y-1">
                                                <div className="flex items-center gap-2">
                                                    <span className="font-medium">Dataset ID: {request.dataset_id}</span>
                                                </div>
                                                <div className="text-sm text-muted-foreground font-mono">Requester: {request.requester}</div>
                                                {request.message && <div className="text-sm text-muted-foreground">{request.message}</div>}
                                            </div>
                                            <div className="flex gap-2">
                                                <Button
                                                    size="sm"
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        handleGrantAccess(request.dataset_id, request.requester);
                                                    }}
                                                    disabled={loading}
                                                >
                                                    Grant
                                                </Button>
                                            </div>
                                        </div>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    )}
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>Grant Access</CardTitle>
                    <CardDescription>Grant access to a requester for your dataset</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <Label htmlFor="grantDatasetId">Dataset ID</Label>
                        <Input
                            id="grantDatasetId"
                            type="number"
                            placeholder="Enter dataset ID"
                            value={datasetId}
                            onChange={(e) => setDatasetId(e.target.value)}
                        />
                    </div>
                    <div>
                        <Label htmlFor="requester">Requester Address</Label>
                        <Input
                            id="requester"
                            placeholder="0x..."
                            value={requester}
                            onChange={(e) => setRequester(e.target.value)}
                            className="font-mono"
                        />
                    </div>
                    <div>
                        <Label htmlFor="expiresAt">Expires At</Label>
                        <Input id="expiresAt" type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
                    </div>
                    <Button
                        onClick={(e) => {
                            e.preventDefault();
                            handleGrantAccess();
                        }}
                        disabled={loading}
                        className="w-full"
                    >
                        {loading ? "Granting..." : "Grant Access"}
                    </Button>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>Revoke Access</CardTitle>
                    <CardDescription>Revoke access from a requester</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <Label htmlFor="revokeDatasetId">Dataset ID</Label>
                        <Input
                            id="revokeDatasetId"
                            type="number"
                            placeholder="Enter dataset ID"
                            value={datasetId}
                            onChange={(e) => setDatasetId(e.target.value)}
                        />
                    </div>
                    <div>
                        <Label htmlFor="revokeRequester">Requester Address</Label>
                        <Input
                            id="revokeRequester"
                            placeholder="0x..."
                            value={requester}
                            onChange={(e) => setRequester(e.target.value)}
                            className="font-mono"
                        />
                    </div>
                    <Button onClick={handleRevokeAccess} disabled={loading} variant="destructive" className="w-full">
                        {loading ? "Revoking..." : "Revoke Access"}
                    </Button>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>Check Access</CardTitle>
                    <CardDescription>Check if a requester has access to a dataset</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <Label htmlFor="checkOwner">Owner Address</Label>
                        <Input
                            id="checkOwner"
                            placeholder="0x..."
                            value={checkOwner}
                            onChange={(e) => setCheckOwner(e.target.value)}
                            className="font-mono"
                        />
                    </div>
                    <div>
                        <Label htmlFor="checkDatasetId">Dataset ID</Label>
                        <Input
                            id="checkDatasetId"
                            type="number"
                            placeholder="Enter dataset ID"
                            value={checkDatasetId}
                            onChange={(e) => setCheckDatasetId(e.target.value)}
                        />
                    </div>
                    <div>
                        <Label htmlFor="checkRequester">Requester Address</Label>
                        <Input
                            id="checkRequester"
                            placeholder="0x..."
                            value={checkRequester}
                            onChange={(e) => setCheckRequester(e.target.value)}
                            className="font-mono"
                        />
                    </div>
                    <Button onClick={handleCheckAccess} disabled={loading} className="w-full">
                        {loading ? "Checking..." : "Check Access"}
                    </Button>
                    {accessResult !== null && (
                        <div className={`p-4 rounded-md ${accessResult ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800"}`}>
                            {accessResult ? "✓ Has Access" : "✗ No Access"}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
