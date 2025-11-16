"use client";

import { FileCheckIcon, InfoIcon, Loader2Icon, OctagonIcon, TriangleIcon } from "lucide-react";
import { useTheme } from "next-themes";
import { Toaster as Sonner, type ToasterProps } from "sonner";

const Toaster = ({ ...props }: ToasterProps) => {
    const { theme = "system" } = useTheme();

    return (
        <Sonner
            theme={theme as ToasterProps["theme"]}
            className="toaster group"
            icons={{
                success: <FileCheckIcon className="size-4" />,
                info: <InfoIcon className="size-4" />,
                warning: <TriangleIcon className="size-4" />,
                error: <OctagonIcon className="size-4" />,
                loading: <Loader2Icon className="size-4 animate-spin" />,
            }}
            toastOptions={{
                classNames: {
                    toast: "bg-popover text-popover-foreground border-border",
                    title: "text-popover-foreground",
                    description: "text-popover-foreground/90",
                    actionButton: "bg-primary text-primary-foreground",
                    cancelButton: "bg-muted text-muted-foreground",
                },
            }}
            style={
                {
                    "--normal-bg": "hsl(var(--popover))",
                    "--normal-text": "hsl(var(--popover-foreground))",
                    "--normal-border": "hsl(var(--border))",
                    "--border-radius": "var(--radius)",
                } as React.CSSProperties
            }
            {...props}
        />
    );
};

export { Toaster };
