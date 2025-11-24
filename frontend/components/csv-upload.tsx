"use client";

import { useState, useRef } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { parseCSV, generateDataHash, type CSVSchema } from "@/lib/csv-parser";
import { apiClient } from "@/lib/api";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { buildTransaction } from "@/lib/aptos-client";
import { Upload, FileText, Hash, Send, CheckCircle, AlertCircle } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { DATAX_MODULE_ADDRESS } from "@/constants";

interface CSVUploadProps {
    account: string;
}

export function CSVUpload({ account }: CSVUploadProps) {
    const [file, setFile] = useState<File | null>(null);
    const [schema, setSchema] = useState<CSVSchema | null>(null);
    const [csvData, setCsvData] = useState<string[][] | null>(null);
    const [dataHash, setDataHash] = useState<string>("");
    const [hashExists, setHashExists] = useState<boolean>(false);
    const [checkingHash, setCheckingHash] = useState<boolean>(false);
    const [description, setDescription] = useState<string>("");
    const [loading, setLoading] = useState(false);
    const [uploading, setUploading] = useState(false);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const { signAndSubmitTransaction } = useWallet();

    const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const selectedFile = e.target.files?.[0];
        if (!selectedFile) return;

        if (!selectedFile.name.endsWith(".csv")) {
            toast.error("Please select a CSV file");
            return;
        }

        setFile(selectedFile);
        setLoading(true);
        setHashExists(false);

        try {
            const { data, schema: parsedSchema } = await parseCSV(selectedFile);
            setCsvData(data);
            setSchema(parsedSchema);

            // Generate and display the data hash immediately
            const hash = await generateDataHash(data);
            setDataHash(hash);

            toast.success(`CSV parsed successfully. Found ${parsedSchema.rowCount} rows and ${parsedSchema.columns.length} columns.`);

            // Check if hash already exists on blockchain
            setCheckingHash(true);
            try {
                const exists = await apiClient.checkDataHash(hash);
                setHashExists(exists);
                if (exists) {
                    toast.error("⚠️ This dataset already exists on the blockchain!");
                }
            } catch (checkError) {
                console.error("Failed to check hash existence:", checkError);
                toast.warning("Could not verify if dataset exists. Proceed with caution.");
            } finally {
                setCheckingHash(false);
            }
        } catch (error: any) {
            toast.error(error.message || "Failed to parse CSV file");
            setFile(null);
            setDataHash("");
            setHashExists(false);
        } finally {
            setLoading(false);
        }
    };

    const handleSubmit = async () => {
        if (!csvData || !schema || !account) {
            toast.error("Please upload a CSV file first");
            return;
        }

        if (!signAndSubmitTransaction) {
            toast.error("Wallet does not support transaction signing");
            return;
        }

        setUploading(true);

        try {
            // Use the pre-generated hash (or generate if not set)
            const hash = dataHash || (await generateDataHash(csvData));
            if (!dataHash) {
                setDataHash(hash);
            }

            // Convert hex string to bytes
            const hexString = hash.startsWith("0x") ? hash.slice(2) : hash;
            const hashBytes = new Uint8Array(hexString.match(/.{1,2}/g)?.map((byte) => parseInt(byte, 16)) || []);

            // Create metadata with schema info and description
            const metadata = JSON.stringify({
                schema: schema.columns.map((col) => ({
                    name: col.name,
                    type: col.type,
                })),
                rowCount: schema.rowCount,
                uploadedAt: new Date().toISOString(),
                description: description || "",
            });
            const encoder = new TextEncoder();
            const metadataBytes = encoder.encode(metadata);

            // Encrypt CSV data client-side before sending to backend
            // Use dynamic import to ensure it only loads in browser
            const encryptionModule = await import("../lib/encryption");
            const { encryptCSVData } = encryptionModule;
            // Properly format CSV with escaping (handle commas, quotes, newlines in cells)
            const formatCSVRow = (row: string[]): string => {
                return row
                    .map((cell) => {
                        // Escape quotes by doubling them, wrap in quotes if contains comma, quote, or newline
                        if (cell.includes(",") || cell.includes('"') || cell.includes("\n")) {
                            return `"${cell.replace(/"/g, '""')}"`;
                        }
                        return cell;
                    })
                    .join(",");
            };
            const csvString = csvData.map(formatCSVRow).join("\n");
            const { encryptedData, metadata: encryptionMetadata } = await encryptCSVData(csvString, account);

            // Encode encryption metadata for on-chain storage
            const encryptionMetadataBytes = encoder.encode(encryptionMetadata);
            const encryptionAlgorithmBytes = encoder.encode("AES-256-GCM");

            // Build transaction with encryption metadata
            const transaction = await buildTransaction(
                {
                    moduleAddress: DATAX_MODULE_ADDRESS, // DataX module address
                    moduleName: "data_registry",
                    functionName: "submit_data",
                    args: [hashBytes, metadataBytes, encryptionMetadataBytes, encryptionAlgorithmBytes],
                },
                account
            );

            // Sign and submit transaction to blockchain (frontend handles this)
            const response = await signAndSubmitTransaction(transaction);

            if (!response || !response.hash) {
                throw new Error("Transaction submission failed - no transaction hash received");
            }

            console.log("Transaction submitted to blockchain:", response.hash);

            // Send encrypted data to backend (no private key needed - frontend already submitted to blockchain)
            try {
                await apiClient.submitEncryptedCSV(account, encryptedData, encryptionMetadata, schema, hash);
                console.log("Encrypted data stored in backend successfully");
            } catch (backendError) {
                console.warn("Backend submission failed:", backendError);
                // Transaction is already on-chain, so this is not critical
                // But we should still show a warning
                toast.warning("Data submitted to blockchain, but backend storage failed. Transaction: " + response.hash);
            }

            toast.success(`Data submitted successfully! Transaction: ${response.hash}`);

            // Reset
            setFile(null);
            setSchema(null);
            setCsvData(null);
            if (fileInputRef.current) {
                fileInputRef.current.value = "";
            }
        } catch (error: any) {
            toast.error(error.message || "Failed to submit data");
        } finally {
            setUploading(false);
        }
    };

    return (
        <Card className="bg-white/5 backdrop-blur-md border-white/10 overflow-hidden">
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Upload className="w-5 h-5 text-blue-400" />
                    Upload CSV Data
                </CardTitle>
                <CardDescription>Upload a CSV file to automatically generate a hash and submit it to the blockchain.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                <div className="relative group">
                    <div
                        className={`border-2 border-dashed rounded-xl p-8 text-center transition-all ${
                            file ? "border-blue-500/50 bg-blue-500/5" : "border-white/10 hover:border-white/20 hover:bg-white/5"
                        }`}
                    >
                        <Input
                            id="csv-file"
                            type="file"
                            accept=".csv"
                            ref={fileInputRef}
                            onChange={handleFileSelect}
                            disabled={loading}
                            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                        />

                        <div className="flex flex-col items-center justify-center gap-3">
                            {file ? (
                                <>
                                    <div className="p-3 bg-blue-500/20 rounded-full">
                                        <FileText className="w-8 h-8 text-blue-400" />
                                    </div>
                                    <div>
                                        <p className="font-medium text-white">{file.name}</p>
                                        <p className="text-xs text-gray-400">{(file.size / 1024).toFixed(2)} KB</p>
                                    </div>
                                    <Button variant="ghost" size="sm" className="z-20 text-blue-400 hover:text-blue-300">
                                        Change File
                                    </Button>
                                </>
                            ) : (
                                <>
                                    <div className="p-3 bg-white/5 rounded-full group-hover:scale-110 transition-transform">
                                        <Upload className="w-8 h-8 text-gray-400 group-hover:text-white transition-colors" />
                                    </div>
                                    <div>
                                        <p className="font-medium text-gray-300 group-hover:text-white">Click to upload or drag and drop</p>
                                        <p className="text-xs text-gray-500">CSV files only</p>
                                    </div>
                                </>
                            )}
                        </div>
                    </div>
                </div>

                {loading && (
                    <div className="flex items-center justify-center gap-2 text-sm text-blue-400 animate-pulse">
                        <div className="w-2 h-2 bg-blue-400 rounded-full animate-bounce" />
                        <div className="w-2 h-2 bg-blue-400 rounded-full animate-bounce delay-75" />
                        <div className="w-2 h-2 bg-blue-400 rounded-full animate-bounce delay-150" />
                        Parsing CSV file...
                    </div>
                )}

                <AnimatePresence>
                    {schema && (
                        <motion.div
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: "auto" }}
                            exit={{ opacity: 0, height: 0 }}
                            className="space-y-6"
                        >
                            {dataHash && (
                                <div
                                    className={`rounded-lg p-4 border ${
                                        hashExists ? "bg-red-500/10 border-red-500/30" : "bg-black/20 border-white/5"
                                    }`}
                                >
                                    <Label className="text-xs text-gray-500 uppercase tracking-wider font-semibold mb-2 block flex items-center gap-2">
                                        <Hash className="w-3 h-3" /> Data Hash (Generated)
                                        {checkingHash && <span className="text-blue-400 ml-auto">Checking...</span>}
                                    </Label>
                                    <div className="flex items-center gap-2">
                                        <code
                                            className={`flex-1 bg-black/40 p-2 rounded text-xs font-mono break-all border border-white/5 ${
                                                hashExists ? "text-red-400" : "text-green-400"
                                            }`}
                                        >
                                            {dataHash}
                                        </code>
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            className="h-8 w-8 p-0"
                                            onClick={() => {
                                                navigator.clipboard.writeText(dataHash);
                                                toast.success("Hash copied");
                                            }}
                                        >
                                            <CheckCircle className="w-4 h-4 text-gray-400 hover:text-white" />
                                        </Button>
                                    </div>
                                    {hashExists ? (
                                        <p className="text-xs text-red-400 mt-2 flex items-center gap-1 font-medium">
                                            <AlertCircle className="w-3 h-3" />
                                            ⚠️ This dataset already exists on the blockchain! You cannot submit it again.
                                        </p>
                                    ) : (
                                        <p className="text-xs text-gray-500 mt-2 flex items-center gap-1">
                                            <AlertCircle className="w-3 h-3" />
                                            This hash will be submitted to the blockchain to prove ownership.
                                        </p>
                                    )}
                                </div>
                            )}

                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div>
                                    <Label className="text-gray-300 mb-2 block">Schema Preview</Label>
                                    <div className="bg-black/20 rounded-lg p-4 border border-white/5 h-full">
                                        <div className="flex items-center justify-between mb-3">
                                            <span className="text-sm font-medium text-white">Columns</span>
                                            <span className="text-xs bg-blue-500/20 text-blue-300 px-2 py-0.5 rounded-full">
                                                {schema.columns.length} total
                                            </span>
                                        </div>
                                        <div className="space-y-2 max-h-40 overflow-y-auto pr-2 custom-scrollbar">
                                            {schema.columns.map((col, idx) => (
                                                <div
                                                    key={idx}
                                                    className="text-sm flex justify-between items-center p-2 bg-white/5 rounded hover:bg-white/10 transition-colors"
                                                >
                                                    <span className="font-medium text-gray-300">{col.name}</span>
                                                    <span className="text-xs text-gray-500 font-mono">{col.type}</span>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <Label htmlFor="description" className="text-gray-300 mb-2 block">
                                        Description (Optional)
                                    </Label>
                                    <Textarea
                                        id="description"
                                        placeholder="Describe your dataset..."
                                        value={description}
                                        onChange={(e) => setDescription(e.target.value)}
                                        className="bg-black/20 border-white/10 focus:border-blue-500/50 h-[calc(100%-28px)] resize-none"
                                    />
                                </div>
                            </div>

                            <div>
                                <Label className="text-gray-300 mb-2 block">Data Preview (first 5 rows)</Label>
                                <div className="overflow-x-auto rounded-lg border border-white/5">
                                    <table className="min-w-full text-sm">
                                        <thead>
                                            <tr className="bg-white/5 border-b border-white/5">
                                                {schema.preview[0]?.map((header, idx) => (
                                                    <th key={idx} className="px-4 py-2 text-left font-medium text-gray-300 whitespace-nowrap">
                                                        {header}
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-white/5">
                                            {schema.preview.slice(1, 6).map((row, rowIdx) => (
                                                <tr key={rowIdx} className="hover:bg-white/5 transition-colors">
                                                    {row.map((cell, cellIdx) => (
                                                        <td
                                                            key={cellIdx}
                                                            className="px-4 py-2 text-gray-400 whitespace-nowrap max-w-[200px] truncate"
                                                        >
                                                            {cell}
                                                        </td>
                                                    ))}
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                                <p className="text-xs text-gray-500 mt-2 text-right">Showing 5 of {schema.rowCount} rows</p>
                            </div>

                            <Button
                                onClick={handleSubmit}
                                disabled={uploading || hashExists || checkingHash}
                                className={`w-full border-0 h-12 text-lg shadow-lg ${
                                    hashExists
                                        ? "bg-gray-600 cursor-not-allowed"
                                        : "bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 shadow-blue-500/20"
                                } text-white`}
                            >
                                {checkingHash ? (
                                    <>
                                        <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin mr-2" />
                                        Verifying Hash...
                                    </>
                                ) : uploading ? (
                                    <>
                                        <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin mr-2" />
                                        Submitting to Blockchain...
                                    </>
                                ) : hashExists ? (
                                    <>
                                        <AlertCircle className="w-5 h-5 mr-2" />
                                        Dataset Already Exists
                                    </>
                                ) : (
                                    <>
                                        <Send className="w-5 h-5 mr-2" />
                                        Submit Data to Blockchain
                                    </>
                                )}
                            </Button>
                        </motion.div>
                    )}
                </AnimatePresence>
            </CardContent>
        </Card>
    );
}
