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

interface CSVUploadProps {
    account: string;
}

export function CSVUpload({ account }: CSVUploadProps) {
    const [file, setFile] = useState<File | null>(null);
    const [schema, setSchema] = useState<CSVSchema | null>(null);
    const [csvData, setCsvData] = useState<string[][] | null>(null);
    const [dataHash, setDataHash] = useState<string>("");
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

        try {
            const { data, schema: parsedSchema } = await parseCSV(selectedFile);
            setCsvData(data);
            setSchema(parsedSchema);

            // Generate and display the data hash immediately
            const hash = await generateDataHash(data);
            setDataHash(hash);

            toast.success(`CSV parsed successfully. Found ${parsedSchema.rowCount} rows and ${parsedSchema.columns.length} columns.`);
        } catch (error: any) {
            toast.error(error.message || "Failed to parse CSV file");
            setFile(null);
            setDataHash("");
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

            // Build transaction
            const transaction = await buildTransaction(
                {
                    moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab", // DataX module address
                    moduleName: "data_registry",
                    functionName: "submit_data",
                    args: [hashBytes, metadataBytes],
                },
                account
            );

            // Sign and submit transaction
            const response = await signAndSubmitTransaction(transaction);

            // Also send to backend for processing (send the actual file)
            try {
                if (file) {
                    await apiClient.submitCSV(account, file, schema, hash);
                }
            } catch (backendError) {
                console.warn("Backend submission failed:", backendError);
                // Transaction is already on-chain, so this is not critical
            }

            toast.success(`Data submitted! Transaction: ${response.hash}`);

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
        <div className="space-y-4">
            <Card>
                <CardHeader>
                    <CardTitle>Upload CSV Data</CardTitle>
                    <CardDescription>
                        Upload a CSV file to automatically generate a hash and submit it to the blockchain. The hash is generated from your CSV
                        content and stored on-chain.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <Label htmlFor="csv-file">CSV File</Label>
                        <Input id="csv-file" type="file" accept=".csv" ref={fileInputRef} onChange={handleFileSelect} disabled={loading} />
                    </div>

                    {loading && <p className="text-sm text-muted-foreground">Parsing CSV file...</p>}

                    {schema && (
                        <div className="space-y-4">
                            {dataHash && (
                                <div>
                                    <Label>Data Hash (Generated)</Label>
                                    <Input
                                        value={dataHash}
                                        readOnly
                                        className="font-mono text-sm"
                                        onClick={(e) => (e.target as HTMLInputElement).select()}
                                    />
                                    <p className="text-xs text-muted-foreground mt-1">
                                        This hash will be submitted to the blockchain. Click to copy.
                                    </p>
                                </div>
                            )}
                            <div>
                                <Label>Schema Preview</Label>
                                <div className="mt-2 p-4 bg-muted rounded-md">
                                    <div className="space-y-2">
                                        <p className="text-sm font-medium">
                                            {schema.rowCount} rows, {schema.columns.length} columns
                                        </p>
                                        <div className="space-y-1">
                                            {schema.columns.map((col, idx) => (
                                                <div key={idx} className="text-sm">
                                                    <span className="font-medium">{col.name}</span>
                                                    <span className="text-muted-foreground ml-2">({col.type})</span>
                                                    {col.sampleValues.length > 0 && (
                                                        <span className="text-muted-foreground ml-2">- Sample: {col.sampleValues[0]}</span>
                                                    )}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div>
                                <Label>Data Preview (first 5 rows)</Label>
                                <div className="mt-2 overflow-x-auto">
                                    <table className="min-w-full text-sm border rounded">
                                        <thead>
                                            <tr className="bg-muted">
                                                {schema.preview[0]?.map((header, idx) => (
                                                    <th key={idx} className="px-2 py-1 text-left border">
                                                        {header}
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {schema.preview.slice(1, 6).map((row, rowIdx) => (
                                                <tr key={rowIdx}>
                                                    {row.map((cell, cellIdx) => (
                                                        <td key={cellIdx} className="px-2 py-1 border">
                                                            {cell}
                                                        </td>
                                                    ))}
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </div>

                            <div>
                                <Label htmlFor="description">Description (Optional)</Label>
                                <Textarea
                                    id="description"
                                    placeholder="Describe your dataset..."
                                    value={description}
                                    onChange={(e) => setDescription(e.target.value)}
                                    rows={3}
                                />
                                <p className="text-xs text-muted-foreground mt-1">Add a description to help others understand your dataset</p>
                            </div>

                            <Button onClick={handleSubmit} disabled={uploading} className="w-full">
                                {uploading ? "Submitting..." : "Submit Data to Blockchain"}
                            </Button>
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
