"use client";

import { useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";

export function AnimatedBackground() {
    const [mousePosition, setMousePosition] = useState({ x: 0, y: 0 });
    
    useEffect(() => {
        const handleMouseMove = (e: MouseEvent) => {
            setMousePosition({
                x: e.clientX,
                y: e.clientY,
            });
        };

        window.addEventListener("mousemove", handleMouseMove);
        return () => window.removeEventListener("mousemove", handleMouseMove);
    }, []);

    return (
        <div className="fixed inset-0 -z-10 overflow-hidden bg-background">
            <div className="absolute inset-0 bg-[radial-gradient(circle_500px_at_50%_200px,#3b82f6,transparent)] opacity-20 dark:opacity-30" />
            <div 
                className="absolute inset-0 opacity-30 dark:opacity-40"
                style={{
                    backgroundImage: `radial-gradient(circle at ${mousePosition.x}px ${mousePosition.y}px, rgba(120, 119, 198, 0.3) 0%, transparent 20%)`,
                }}
            />
            <div className="absolute inset-0 bg-[url('/grid.svg')] bg-center [mask-image:linear-gradient(180deg,white,rgba(255,255,255,0))]" />
            
            {/* Floating orbs */}
            <motion.div 
                animate={{ 
                    x: [0, 100, 0],
                    y: [0, -50, 0],
                    scale: [1, 1.2, 1]
                }}
                transition={{ 
                    duration: 20, 
                    repeat: Infinity,
                    ease: "easeInOut" 
                }}
                className="absolute top-1/4 left-1/4 w-96 h-96 bg-purple-500/30 rounded-full blur-3xl"
            />
            <motion.div 
                animate={{ 
                    x: [0, -100, 0],
                    y: [0, 50, 0],
                    scale: [1, 1.5, 1]
                }}
                transition={{ 
                    duration: 25, 
                    repeat: Infinity,
                    ease: "easeInOut",
                    delay: 2
                }}
                className="absolute top-1/3 right-1/4 w-64 h-64 bg-blue-500/30 rounded-full blur-3xl"
            />
             <motion.div 
                animate={{ 
                    x: [0, 50, 0],
                    y: [0, 100, 0],
                    scale: [1, 1.3, 1]
                }}
                transition={{ 
                    duration: 18, 
                    repeat: Infinity,
                    ease: "easeInOut",
                    delay: 5
                }}
                className="absolute bottom-1/4 left-1/3 w-80 h-80 bg-indigo-500/30 rounded-full blur-3xl"
            />
        </div>
    );
}
