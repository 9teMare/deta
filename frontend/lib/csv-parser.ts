export interface CSVSchema {
    columns: {
        name: string;
        type: "string" | "number" | "date" | "boolean";
        sampleValues: string[];
    }[];
    rowCount: number;
    preview: string[][];
}

export function parseCSV(file: File): Promise<{ data: string[][]; schema: CSVSchema }> {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();

        reader.onload = (e) => {
            try {
                const text = e.target?.result as string;
                const lines = text.split("\n").filter((line) => line.trim());

                if (lines.length === 0) {
                    reject(new Error("CSV file is empty"));
                    return;
                }

                // Parse CSV (handling quoted values)
                const parseCSVLine = (line: string): string[] => {
                    const result: string[] = [];
                    let current = "";
                    let inQuotes = false;

                    for (let i = 0; i < line.length; i++) {
                        const char = line[i];
                        if (char === '"') {
                            inQuotes = !inQuotes;
                        } else if (char === "," && !inQuotes) {
                            result.push(current.trim());
                            current = "";
                        } else {
                            current += char;
                        }
                    }
                    result.push(current.trim());
                    return result;
                };

                const rows = lines.map(parseCSVLine);
                const headers = rows[0];
                const dataRows = rows.slice(1);

                // Infer schema
                const schema: CSVSchema = {
                    columns: headers.map((header, colIndex) => {
                        const sampleValues = dataRows
                            .slice(0, Math.min(10, dataRows.length))
                            .map((row) => row[colIndex] || "")
                            .filter((val) => val !== "");

                        // Infer type
                        let type: "string" | "number" | "date" | "boolean" = "string";
                        if (sampleValues.length > 0) {
                            const firstValue = sampleValues[0];
                            if (!isNaN(Number(firstValue)) && firstValue !== "") {
                                type = "number";
                            } else if (/^\d{4}-\d{2}-\d{2}/.test(firstValue) || /^\d{2}\/\d{2}\/\d{4}/.test(firstValue)) {
                                type = "date";
                            } else if (firstValue.toLowerCase() === "true" || firstValue.toLowerCase() === "false") {
                                type = "boolean";
                            }
                        }

                        return {
                            name: header,
                            type,
                            sampleValues: sampleValues.slice(0, 5),
                        };
                    }),
                    rowCount: dataRows.length,
                    preview: rows.slice(0, Math.min(11, rows.length)),
                };

                resolve({ data: rows, schema });
            } catch (error) {
                reject(error);
            }
        };

        reader.onerror = () => reject(new Error("Failed to read file"));
        reader.readAsText(file);
    });
}

export async function generateDataHash(csvData: string[][]): Promise<string> {
    // Generate a hash of the CSV data using Web Crypto API
    const dataString = JSON.stringify(csvData);
    const encoder = new TextEncoder();
    const data = encoder.encode(dataString);

    // Use Web Crypto API for hashing
    const hashBuffer = await crypto.subtle.digest("SHA-256", data);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");

    return `0x${hashHex}`;
}
