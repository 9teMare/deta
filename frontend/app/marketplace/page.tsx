"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { apiClient } from "@/lib/api";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { buildTransaction } from "@/lib/aptos-client";
import Link from "next/link";
import { Database, Users, Calendar, FileText, ExternalLink, Search, Filter, DollarSign } from "lucide-react";
import { createAccessRequest, getApprovedRequests, confirmPayment, checkRequestExists, type AccessRequest } from "@/lib/supabase";

interface MarketplaceDataset {
    id: number;
    owner: string;
    data_hash: string;
    metadata: string;
    created_at: number;
    is_active: boolean;
    description?: string;
    schema?: any;
    rowCount?: number;
}

export default function MarketplacePage() {
    const { account, connected, signAndSubmitTransaction } = useWallet();
    const [datasets, setDatasets] = useState<MarketplaceDataset[]>([]);
    const [loading, setLoading] = useState(false);
    const [selectedDataset, setSelectedDataset] = useState<MarketplaceDataset | null>(null);
    const [requestingAccess, setRequestingAccess] = useState(false);
    const [viewingCSV, setViewingCSV] = useState<{ dataset: MarketplaceDataset; data: string[][] } | null>(null);
    const [loadingCSV, setLoadingCSV] = useState(false);
    const [accessChecks, setAccessChecks] = useState<Map<string, boolean>>(new Map());
    const [showOnlyOthers, setShowOnlyOthers] = useState(true);
    const [searchQuery, setSearchQuery] = useState("");
    const [approvedRequests, setApprovedRequests] = useState<AccessRequest[]>([]);
    const [payingFor, setPayingFor] = useState<string | null>(null);

    const loadMarketplaceDatasets = async () => {
        setLoading(true);
        try {
            const result = await apiClient.getMarketplaceDatasets();
            setDatasets(result);
            
            if (result.length === 0) {
                toast.info("No datasets found. Submit some data first.");
            }
        } catch (error: any) {
            toast.error(error.message || "Failed to load marketplace datasets");
        } finally {
            setLoading(false);
        }
    };

    const loadApprovedRequests = async () => {
        if (!connected || !account) return;
        
        const requests = await getApprovedRequests(account.address.toString());
        setApprovedRequests(requests);
    };

    useEffect(() => {
        loadMarketplaceDatasets();
    }, []);

    useEffect(() => {
        loadApprovedRequests();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [connected, account]);

    // Check access for all datasets when account changes
    useEffect(() => {
        if (connected && account && datasets.length > 0) {
            datasets.forEach(async (dataset) => {
                if (dataset.owner !== account.address.toString()) {
                    await checkAccessForDataset(dataset);
                }
            });
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [connected, account, datasets.length]);

    const checkAccessForDataset = async (dataset: MarketplaceDataset) => {
        if (!connected || !account) return false;
        
        const key = `${dataset.owner}-${dataset.id}`;
        if (accessChecks.has(key)) {
            return accessChecks.get(key)!;
        }

        try {
            const result = await apiClient.checkAccess(dataset.owner, dataset.id, account.address.toString());
            setAccessChecks(new Map(accessChecks.set(key, result.has_access)));
            return result.has_access;
        } catch {
            return false;
        }
    };

    const handleViewCSV = async (dataset: MarketplaceDataset) => {
        if (!connected || !account) {
            toast.error("Please connect your wallet first");
            return;
        }

        setLoadingCSV(true);
        try {
            const csvData = await apiClient.getCSVData(dataset.data_hash, dataset.owner, dataset.id, account.address.toString());
            setViewingCSV({ dataset, data: csvData });
        } catch (error: any) {
            toast.error(error.message || "Failed to load CSV data. You may not have access.");
        } finally {
            setLoadingCSV(false);
        }
    };

    const handleRequestAccess = async (dataset: MarketplaceDataset) => {
        if (!connected || !account) {
            toast.error("Please connect your wallet first");
            return;
        }

        if (dataset.owner === account.address.toString()) {
            toast.error("You cannot request access to your own dataset");
            return;
        }

        setRequestingAccess(true);
        try {
            // Check if request already exists
            const existingRequest = await checkRequestExists(
                dataset.owner,
                account.address.toString(),
                dataset.id
            );

            if (existingRequest) {
                if (existingRequest.status === 'pending') {
                    toast.info("You already have a pending request for this dataset");
                } else if (existingRequest.status === 'approved') {
                    toast.info("Your request was approved! Pay 0.1 APT to access the data.");
                } else if (existingRequest.status === 'paid') {
                    toast.info("You already paid for this dataset. Waiting for owner to grant access.");
                } else if (existingRequest.status === 'denied') {
                    toast.error("Your request was denied by the owner");
                }
                return;
            }

            // Create new access request in database
            const request = await createAccessRequest(
                dataset.owner,
                account.address.toString(),
                dataset.id,
                `Requesting access to dataset #${dataset.id}`
            );

            if (request) {
                toast.success("Access request submitted! The dataset owner will review it.");
            } else {
                toast.error("Failed to create access request");
            }
        } catch (error: any) {
            toast.error(error.message || "Failed to request access");
        } finally {
            setRequestingAccess(false);
        }
    };

    const handlePayForApprovedRequest = async (request: AccessRequest) => {
        if (!connected || !account || !signAndSubmitTransaction) {
            toast.error("Please connect your wallet first");
            return;
        }

        const ACCESS_PRICE_OCTAS = 10000000; // 0.1 APT
        const requestKey = `${request.owner_address}-${request.dataset_id}`;
        
        setPayingFor(requestKey);
        try {
            // Step 1: Transfer APT payment to dataset owner
            const paymentTransaction = {
                data: {
                    function: "0x1::aptos_account::transfer",
                    functionArguments: [request.owner_address, ACCESS_PRICE_OCTAS],
                },
            };

            const paymentResponse = await signAndSubmitTransaction(paymentTransaction);
            toast.success(`Payment sent! Transaction: ${paymentResponse.hash}`);

            // Step 2: Update database with payment confirmation
            const confirmed = await confirmPayment(
                request.owner_address,
                request.requester_address,
                request.dataset_id,
                paymentResponse.hash
            );

            if (!confirmed) {
                toast.error("Failed to confirm payment in database");
                return;
            }

            // Step 3: Trigger on-chain grant access via Aptos
            try {
                const grantTransaction = await buildTransaction(
                    {
                        moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab",
                        moduleName: "AccessControl",
                        functionName: "grant_access",
                        args: [request.dataset_id, request.requester_address, Math.floor(Date.now() / 1000) + 365 * 24 * 60 * 60], // 1 year expiry
                    },
                    request.owner_address // This should be signed by owner, but for auto-grant we need a different approach
                );

                // Note: Since we can't sign on behalf of owner, the owner will need to manually grant access
                // For now, just show success message and owner will see the paid request
                toast.success("Payment confirmed! Waiting for dataset owner to grant access on-chain.");
            } catch (grantError) {
                console.error("Grant access error:", grantError);
                toast.info("Payment confirmed! The owner will grant access shortly.");
            }

            // Reload approved requests
            await loadApprovedRequests();
        } catch (error: any) {
            toast.error(error.message || "Payment failed");
        } finally {
            setPayingFor(null);
        }
    };

    const parseMetadata = (metadata: string) => {
        if (!metadata || metadata.trim() === "") {
            return null;
        }

        try {
            if (metadata.startsWith("[")) {
                return null;
            }
            return JSON.parse(metadata);
        } catch {
            return null;
        }
    };

    const formatDate = (timestamp: number) => {
        if (!timestamp) return "N/A";
        return new Date(timestamp * 1000).toLocaleDateString("en-US", {
            year: "numeric",
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        });
    };

    const truncateAddress = (address: string) => {
        if (!address) return "";
        return `${address.slice(0, 6)}...${address.slice(-4)}`;
    };

    // Filter datasets
    const filteredDatasets = datasets.filter((d) => {
        if (!d.is_active) return false;
        
        // Filter by owner
        if (showOnlyOthers && connected && account) {
            if (d.owner === account.address.toString()) {
                return false;
            }
        }

        // Filter by search query
        if (searchQuery) {
            const query = searchQuery.toLowerCase();
            const metadata = parseMetadata(d.metadata);
            const description = metadata?.description?.toLowerCase() || "";
            const owner = d.owner.toLowerCase();
            const id = d.id.toString();
            
            if (!description.includes(query) && !owner.includes(query) && !id.includes(query)) {
                return false;
            }
        }

        return true;
    });

    return (
        <div className="min-h-screen bg-gradient-to-br from-slate-50 via-blue-50 to-indigo-50 dark:from-gray-950 dark:via-gray-900 dark:to-gray-800">
            <div className="container mx-auto px-4 py-8">
                <div className="max-w-7xl mx-auto">
                    {/* Header */}
                    <div className="mb-8">
                        <div className="flex items-center justify-between mb-4">
                            <div>
                                <h1 className="text-4xl font-bold bg-gradient-to-r from-blue-600 to-indigo-600 bg-clip-text text-transparent mb-2">
                                    Data Marketplace
                                </h1>
                                <p className="text-muted-foreground text-lg">
                                    Discover and request access to datasets from the community
                                </p>
                            </div>
                            <Link href="/">
                                <Button variant="outline" className="gap-2">
                                    <ExternalLink className="h-4 w-4" />
                                    Back to Home
                                </Button>
                            </Link>
                        </div>

                        {/* Stats */}
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-6">
                            <Card>
                                <CardContent className="pt-6">
                                    <div className="flex items-center gap-3">
                                        <div className="p-2 bg-blue-100 dark:bg-blue-900 rounded-lg">
                                            <Database className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                                        </div>
                                        <div>
                                            <p className="text-2xl font-bold">{filteredDatasets.length}</p>
                                            <p className="text-sm text-muted-foreground">Available Datasets</p>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                            <Card>
                                <CardContent className="pt-6">
                                    <div className="flex items-center gap-3">
                                        <div className="p-2 bg-indigo-100 dark:bg-indigo-900 rounded-lg">
                                            <Users className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
                                        </div>
                                        <div>
                                            <p className="text-2xl font-bold">
                                                {new Set(datasets.map(d => d.owner)).size}
                                            </p>
                                            <p className="text-sm text-muted-foreground">Data Providers</p>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                            <Card>
                                <CardContent className="pt-6">
                                    <div className="flex items-center gap-3">
                                        <div className="p-2 bg-green-100 dark:bg-green-900 rounded-lg">
                                            <FileText className="h-5 w-5 text-green-600 dark:text-green-400" />
                                        </div>
                                        <div>
                                            <p className="text-2xl font-bold">
                                                {datasets.reduce((sum, d) => {
                                                    const meta = parseMetadata(d.metadata);
                                                    return sum + (meta?.rowCount || 0);
                                                }, 0).toLocaleString()}
                                            </p>
                                            <p className="text-sm text-muted-foreground">Total Rows</p>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        </div>
                    </div>

                    {/* Filters and Search */}
                    <Card className="mb-6">
                        <CardContent className="pt-6">
                            <div className="flex flex-col md:flex-row gap-4 items-start md:items-center justify-between">
                                <div className="flex-1 w-full md:w-auto">
                                    <div className="relative">
                                        <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                                        <input
                                            type="text"
                                            placeholder="Search datasets by description, owner, or ID..."
                                            value={searchQuery}
                                            onChange={(e) => setSearchQuery(e.target.value)}
                                            className="w-full pl-10 pr-4 py-2 border rounded-md bg-background"
                                        />
                                    </div>
                                </div>
                                <div className="flex items-center gap-4">
                                    {connected && account && (
                                        <div className="flex items-center gap-2">
                                            <Filter className="h-4 w-4 text-muted-foreground" />
                                            <Label htmlFor="filter-others" className="text-sm cursor-pointer">
                                                Show only others&apos; datasets
                                            </Label>
                                            <Switch
                                                id="filter-others"
                                                checked={showOnlyOthers}
                                                onCheckedChange={setShowOnlyOthers}
                                            />
                                        </div>
                                    )}
                                    <Button onClick={loadMarketplaceDatasets} disabled={loading} variant="outline" className="gap-2">
                                        {loading ? "Loading..." : "Refresh"}
                                    </Button>
                                </div>
                            </div>
                        </CardContent>
                    </Card>

                    {/* Loading State */}
                    {loading && (
                        <div className="text-center py-12">
                            <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                            <p className="text-muted-foreground mt-4">Loading datasets from blockchain...</p>
                        </div>
                    )}

                    {/* Empty State */}
                    {!loading && filteredDatasets.length === 0 && (
                        <Card>
                            <CardContent className="pt-12 pb-12">
                                <div className="text-center">
                                    <Database className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                                    <p className="text-lg font-semibold mb-2">No datasets available</p>
                                    <p className="text-muted-foreground">
                                        {searchQuery || showOnlyOthers
                                            ? "Try adjusting your filters or search query"
                                            : "Be the first to submit a dataset to the marketplace!"}
                                    </p>
                                </div>
                            </CardContent>
                        </Card>
                    )}

                    {/* Approved Requests - Awaiting Payment */}
                    {connected && approvedRequests.length > 0 && (
                        <Card className="mb-6 border-2 border-green-500/20 bg-green-500/5">
                            <CardHeader>
                                <div className="flex items-center gap-2">
                                    <DollarSign className="h-5 w-5 text-green-500" />
                                    <CardTitle>Approved Requests - Pay to Access</CardTitle>
                                </div>
                                <CardDescription>
                                    Your access requests have been approved! Pay 0.1 APT to unlock the data.
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <div className="space-y-3">
                                    {approvedRequests.map((request) => (
                                        <Card key={request.id} className="bg-white/5 border-green-500/20">
                                            <CardContent className="pt-4">
                                                <div className="flex justify-between itemscenter gap-4">
                                                    <div className="space-y-1">
                                                        <div className="flex items-center gap-2">
                                                            <Badge variant="secondary">Dataset #{request.dataset_id}</Badge>
                                                            <Badge className="bg-green-500/20 text-green-600">Approved ✓</Badge>
                                                        </div>
                                                        <p className="text-sm text-muted-foreground font-mono">
                                                            Owner: {truncateAddress(request.owner_address)}
                                                        </p>
                                                        <p className="text-sm font-semibold text-green-600">
                                                            Price: 0.1 APT
                                                        </p>
                                                    </div>
                                                    <Button
                                                        onClick={() => handlePayForApprovedRequest(request)}
                                                        disabled={payingFor === `${request.owner_address}-${request.dataset_id}`}
                                                        className="bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-500 hover:to-emerald-500"
                                                    >
                                                        {payingFor === `${request.owner_address}-${request.dataset_id}` 
                                                            ? "Processing..." 
                                                            : "Pay 0.1 APT"}
                                                    </Button>
                                                </div>
                                            </CardContent>
                                        </Card>
                                    ))}
                                </div>
                            </CardContent>
                        </Card>
                    )}

                    {/* Dataset Grid */}
                    {!loading && filteredDatasets.length > 0 && (
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                            {filteredDatasets.map((dataset) => {
                                const metadata = parseMetadata(dataset.metadata);
                                const isOwner = connected && account && dataset.owner === account.address.toString();
                                const hasAccess = accessChecks.get(`${dataset.owner}-${dataset.id}`);

                                return (
                                    <Card
                                        key={`${dataset.owner}-${dataset.id}`}
                                        className="hover:shadow-xl transition-all duration-300 border-2 hover:border-primary/50 group"
                                    >
                                        <CardHeader className="pb-3">
                                            <div className="flex justify-between items-start mb-2">
                                                <div className="flex items-center gap-2">
                                                    <div className="p-2 bg-primary/10 rounded-lg group-hover:bg-primary/20 transition-colors">
                                                        <Database className="h-4 w-4 text-primary" />
                                                    </div>
                                                    <CardTitle className="text-xl">Dataset #{dataset.id}</CardTitle>
                                                </div>
                                                <Badge variant={dataset.is_active ? "default" : "secondary"} className="shrink-0">
                                                    {dataset.is_active ? "Active" : "Inactive"}
                                                </Badge>
                                            </div>
                                            <div className="flex items-center gap-2 text-xs text-muted-foreground">
                                                <Users className="h-3 w-3" />
                                                <span className="font-mono">{truncateAddress(dataset.owner)}</span>
                                                {isOwner && (
                                                    <Badge variant="outline" className="ml-2 text-xs">Yours</Badge>
                                                )}
                                            </div>
                                        </CardHeader>
                                        <CardContent className="space-y-4">
                                            {metadata?.description && (
                                                <div>
                                                    <p className="text-sm text-muted-foreground line-clamp-2">
                                                        {metadata.description}
                                                    </p>
                                                </div>
                                            )}
                                            {metadata?.schema && (
                                                <div className="flex items-center gap-4 text-sm">
                                                    <div className="flex items-center gap-1">
                                                        <FileText className="h-4 w-4 text-muted-foreground" />
                                                        <span className="font-medium">{metadata.schema.length}</span>
                                                        <span className="text-muted-foreground">columns</span>
                                                    </div>
                                                    <div className="flex items-center gap-1">
                                                        <Database className="h-4 w-4 text-muted-foreground" />
                                                        <span className="font-medium">{metadata.rowCount?.toLocaleString() || 0}</span>
                                                        <span className="text-muted-foreground">rows</span>
                                                    </div>
                                                </div>
                                            )}
                                            <div className="flex items-center gap-2 text-xs text-muted-foreground">
                                                <Calendar className="h-3 w-3" />
                                                <span>{formatDate(dataset.created_at)}</span>
                                            </div>
                                            {!isOwner && (
                                                <div className="flex items-center gap-2 mt-2">
                                                    <Badge variant="secondary" className="text-xs">
                                                        💰 0.1 APT
                                                    </Badge>
                                                    <span className="text-xs text-muted-foreground">Access Fee</span>
                                                </div>
                                            )}
                                            <div className="flex gap-2 pt-2">
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    className="flex-1"
                                                    onClick={() => setSelectedDataset(dataset)}
                                                >
                                                    Details
                                                </Button>
                                                {connected && account && !isOwner && (
                                                    <>
                                                        {hasAccess ? (
                                                            <Button
                                                                size="sm"
                                                                className="flex-1"
                                                                onClick={() => handleViewCSV(dataset)}
                                                                disabled={loadingCSV}
                                                            >
                                                                {loadingCSV ? "Loading..." : "View Data"}
                                                            </Button>
                                                        ) : (
                                                            <Button
                                                                size="sm"
                                                                className="flex-1 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500"
                                                                onClick={() => handleRequestAccess(dataset)}
                                                                disabled={requestingAccess}
                                                            >
                                                                {requestingAccess ? "Processing..." : "Pay & Request (0.1 APT)"}
                                                            </Button>
                                                        )}
                                                    </>
                                                )}
                                            </div>
                                        </CardContent>
                                    </Card>
                                );
                            })}
                        </div>
                    )}

                    {/* Dataset Detail Modal */}
                    {selectedDataset && (
                        <div
                            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
                            onClick={() => setSelectedDataset(null)}
                        >
                            <Card
                                className="max-w-2xl w-full max-h-[90vh] overflow-y-auto shadow-2xl"
                                onClick={(e) => e.stopPropagation()}
                            >
                                <CardHeader>
                                    <div className="flex justify-between items-start">
                                        <div>
                                            <CardTitle className="text-2xl mb-2">Dataset #{selectedDataset.id}</CardTitle>
                                            <CardDescription className="font-mono text-sm">
                                                {selectedDataset.owner}
                                            </CardDescription>
                                        </div>
                                        <Button variant="ghost" size="sm" onClick={() => setSelectedDataset(null)}>
                                            ×
                                        </Button>
                                    </div>
                                </CardHeader>
                                <CardContent className="space-y-6">
                                    {(() => {
                                        const metadata = parseMetadata(selectedDataset.metadata);
                                        return (
                                            <>
                                                {metadata?.description && (
                                                    <div>
                                                        <Label className="text-base font-semibold">Description</Label>
                                                        <p className="text-sm mt-2 p-3 bg-muted rounded-md">
                                                            {metadata.description}
                                                        </p>
                                                    </div>
                                                )}
                                                {metadata?.schema && (
                                                    <div>
                                                        <Label className="text-base font-semibold">Schema</Label>
                                                        <div className="mt-2 space-y-2">
                                                            <div className="flex gap-4 text-sm">
                                                                <span>
                                                                    <strong>{metadata.schema.length}</strong> columns
                                                                </span>
                                                                <span>
                                                                    <strong>{metadata.rowCount?.toLocaleString() || 0}</strong> rows
                                                                </span>
                                                            </div>
                                                            <div className="p-3 bg-muted rounded-md">
                                                                <div className="text-xs text-muted-foreground space-y-1">
                                                                    {metadata.schema.map((col: any, idx: number) => (
                                                                        <div key={idx} className="flex justify-between">
                                                                            <span className="font-medium">{col.name}</span>
                                                                            <span className="text-muted-foreground">({col.type})</span>
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            </div>
                                                        </div>
                                                    </div>
                                                )}
                                            </>
                                        );
                                    })()}
                                    <div>
                                        <Label className="text-base font-semibold">Data Hash</Label>
                                        <p className="text-xs font-mono break-all mt-2 p-3 bg-muted rounded-md">
                                            {selectedDataset.data_hash}
                                        </p>
                                    </div>
                                    <div>
                                        <Label className="text-base font-semibold">Created</Label>
                                        <p className="text-sm mt-2">{formatDate(selectedDataset.created_at)}</p>
                                    </div>
                                    {connected && account && selectedDataset.owner !== account.address.toString() && (
                                        <>
                                            {accessChecks.get(`${selectedDataset.owner}-${selectedDataset.id}`) ? (
                                                <Button
                                                    className="w-full"
                                                    onClick={() => {
                                                        handleViewCSV(selectedDataset);
                                                        setSelectedDataset(null);
                                                    }}
                                                    disabled={loadingCSV}
                                                >
                                                    {loadingCSV ? "Loading..." : "View CSV Data"}
                                                </Button>
                                            ) : (
                                                <Button
                                                    className="w-full bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500"
                                                    onClick={() => {
                                                        handleRequestAccess(selectedDataset);
                                                        setSelectedDataset(null);
                                                    }}
                                                    disabled={requestingAccess}
                                                >
                                                    {requestingAccess ? "Processing Payment..." : "Pay 0.1 APT & Request Access"}
                                                </Button>
                                            )}
                                        </>
                                    )}
                                </CardContent>
                            </Card>
                        </div>
                    )}

                    {/* CSV Data Viewer Modal */}
                    {viewingCSV && (
                        <div
                            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
                            onClick={() => setViewingCSV(null)}
                        >
                            <Card
                                className="max-w-6xl w-full max-h-[90vh] overflow-hidden flex flex-col shadow-2xl"
                                onClick={(e) => e.stopPropagation()}
                            >
                                <CardHeader>
                                    <div className="flex justify-between items-start">
                                        <div>
                                            <CardTitle>CSV Data - Dataset #{viewingCSV.dataset.id}</CardTitle>
                                            <CardDescription>{viewingCSV.data.length} rows</CardDescription>
                                        </div>
                                        <Button variant="ghost" size="sm" onClick={() => setViewingCSV(null)}>
                                            ×
                                        </Button>
                                    </div>
                                </CardHeader>
                                <CardContent className="flex-1 overflow-auto">
                                    <div className="overflow-x-auto">
                                        <table className="min-w-full text-sm border rounded-lg">
                                            <thead className="bg-muted sticky top-0">
                                                <tr>
                                                    {viewingCSV.data[0]?.map((header, idx) => (
                                                        <th key={idx} className="px-4 py-3 text-left border font-semibold">
                                                            {header}
                                                        </th>
                                                    ))}
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {viewingCSV.data.slice(1).map((row, rowIdx) => (
                                                    <tr key={rowIdx} className="hover:bg-muted/50 border-b">
                                                        {row.map((cell, cellIdx) => (
                                                            <td key={cellIdx} className="px-4 py-2 border-r last:border-r-0">
                                                                {cell}
                                                            </td>
                                                        ))}
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                </CardContent>
                            </Card>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
