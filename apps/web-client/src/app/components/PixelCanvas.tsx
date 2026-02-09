"use client";

import { useEffect, useRef, useState } from "react";

const GRID_SIZE = 50;
const PIXEL_SIZE = 20;

type PixelData = {
    x: number;
    y: number;
    color: string;
    userId: string;
};

type ChatMessage = {
    type: "chat";
    userId: string;
    message: string;
    timestamp: number;
    isSystem?: boolean;
};

type UserListMessage = {
    type: "user_list";
    users: string[];
};

type UserLeaveMessage = {
    type: "user_leave";
    userId: string;
};

const COLORS = [
    { name: "Red", value: "#EF4444", ring: "ring-red-500" },
    { name: "Orange", value: "#F97316", ring: "ring-orange-500" },
    { name: "Yellow", value: "#EAB308", ring: "ring-yellow-500" },
    { name: "Green", value: "#22C55E", ring: "ring-green-500" },
    { name: "Blue", value: "#3B82F6", ring: "ring-blue-500" },
    { name: "Purple", value: "#A855F7", ring: "ring-purple-500" },
    { name: "Pink", value: "#EC4899", ring: "ring-pink-500" },
    { name: "Black", value: "#000000", ring: "ring-gray-900" },
    { name: "White", value: "#FFFFFF", ring: "ring-gray-300" },
];

export default function PixelCanvas() {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const chatEndRef = useRef<HTMLDivElement>(null);
    const [color, setColor] = useState("#EF4444");
    const [socket, setSocket] = useState<WebSocket | null>(null);
    const [isDrawing, setIsDrawing] = useState(false);
    const [users, setUsers] = useState<string[]>([]);
    const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
    const [chatInput, setChatInput] = useState("");
    const [showChat, setShowChat] = useState(true);
    const [showUsers, setShowUsers] = useState(true);
    const [myUserId] = useState(() => {
        const adjectives = ["Happy", "Sleepy", "Grumpy", "Dopey", "Sneezy", "Bashful", "Doc"];
        const nouns = ["Panda", "Tiger", "Lion", "Bear", "Fox", "Wolf", "Eagle"];
        return `${adjectives[Math.floor(Math.random() * adjectives.length)]}${nouns[Math.floor(Math.random() * nouns.length)]}`;
    });

    useEffect(() => {
        const gatewayUrl = process.env.NEXT_PUBLIC_GATEWAY_URL || 'ws://localhost:8080';
        const ws = new WebSocket(`${gatewayUrl}/ws?userId=${myUserId}`);

        ws.onopen = () => {
            console.log("Connected to WebSocket");
            // Wait a bit for connection to stabilize
            setTimeout(() => {
                // Request initial canvas state
                ws.send(JSON.stringify({ type: "request_canvas" }));
                // Request chat history
                ws.send(JSON.stringify({ type: "request_chat" }));
            }, 100);
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                
                if (data.type === "user_list") {
                    const msg = data as UserListMessage;
                    setUsers(msg.users);
                } else if (data.type === "user_leave") {
                    const msg = data as UserLeaveMessage;
                    setUsers(prev => prev.filter(u => u !== msg.userId));
                } else if (data.type === "chat") {
                    const msg = data as ChatMessage;
                    setChatMessages(prev => [...prev, msg]);
                } else if (data.type === "chat_history") {
                    // Load chat history
                    const messages = data.messages as ChatMessage[];
                    // Sort by timestamp
                    messages.sort((a, b) => a.timestamp - b.timestamp);
                    setChatMessages(messages);
                } else if (data.type === "clear_chat") {
                    // Clear chat messages
                    setChatMessages([]);
                } else if (data.type === "canvas_state") {
                    // Load initial canvas state
                    const pixels = data.pixels as PixelData[];
                    pixels.forEach(pixel => {
                        drawPixel(pixel.x, pixel.y, pixel.color);
                    });
                } else if (data.type === "clear_canvas") {
                    // Someone cleared the canvas
                    handleReset();
                } else {
                    // Pixel data
                    const pixelData = data as PixelData;
                    drawPixel(pixelData.x, pixelData.y, pixelData.color);
                }
            } catch (e) {
                console.error("Failed to parse message", e);
            }
        };

        setSocket(ws);

        return () => {
            ws.close();
        };
    }, [myUserId]);

    useEffect(() => {
        // Auto scroll to bottom when new chat message arrives
        if (chatEndRef.current) {
            chatEndRef.current.scrollIntoView({ behavior: "smooth", block: "nearest" });
        }
    }, [chatMessages]);

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        ctx.fillStyle = "#FFFFFF";
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        ctx.strokeStyle = "#E5E7EB";
        ctx.lineWidth = 1;

        for (let i = 0; i <= GRID_SIZE; i++) {
            ctx.beginPath();
            ctx.moveTo(i * PIXEL_SIZE, 0);
            ctx.lineTo(i * PIXEL_SIZE, canvas.height);
            ctx.stroke();

            ctx.beginPath();
            ctx.moveTo(0, i * PIXEL_SIZE);
            ctx.lineTo(canvas.width, i * PIXEL_SIZE);
            ctx.stroke();
        }
    }, []);

    const drawPixel = (x: number, y: number, color: string) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        ctx.fillStyle = color;
        ctx.fillRect(x * PIXEL_SIZE, y * PIXEL_SIZE, PIXEL_SIZE, PIXEL_SIZE);
    };

    const getPixelCoordinates = (e: React.MouseEvent<HTMLCanvasElement>) => {
        if (!canvasRef.current) return null;
        const rect = canvasRef.current.getBoundingClientRect();
        const x = Math.floor((e.clientX - rect.left) / PIXEL_SIZE);
        const y = Math.floor((e.clientY - rect.top) / PIXEL_SIZE);
        return { x, y };
    };

    const sendPixel = (x: number, y: number) => {
        if (!socket) return;
        
        drawPixel(x, y, color);

        socket.send(JSON.stringify({
            type: "pixel",
            payload: {
                x,
                y,
                color,
                userId: myUserId
            }
        }));
    };

    const sendChatMessage = () => {
        if (!socket || !chatInput.trim()) return;

        socket.send(JSON.stringify({
            type: "chat",
            payload: {
                message: chatInput.trim()
            }
        }));

        setChatInput("");
    };

    const clearChat = () => {
        if (!socket) return;
        
        if (confirm("정말로 채팅을 초토화 하시겠습니까? 💣")) {
            socket.send(JSON.stringify({
                type: "clear_chat",
                payload: {}
            }));
        }
    };

    const handleChatKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Enter") {
            sendChatMessage();
        }
    };

    const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
        setIsDrawing(true);
        const coords = getPixelCoordinates(e);
        if (coords) {
            sendPixel(coords.x, coords.y);
        }
    };

    const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
        if (!isDrawing) return;
        const coords = getPixelCoordinates(e);
        if (coords) {
            sendPixel(coords.x, coords.y);
        }
    };

    const handleMouseUp = () => {
        setIsDrawing(false);
    };

    const handleMouseLeave = () => {
        setIsDrawing(false);
    };

    const handleReset = () => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        ctx.fillStyle = "#FFFFFF";
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        ctx.strokeStyle = "#E5E7EB";
        ctx.lineWidth = 1;

        for (let i = 0; i <= GRID_SIZE; i++) {
            ctx.beginPath();
            ctx.moveTo(i * PIXEL_SIZE, 0);
            ctx.lineTo(i * PIXEL_SIZE, canvas.height);
            ctx.stroke();

            ctx.beginPath();
            ctx.moveTo(0, i * PIXEL_SIZE);
            ctx.lineTo(canvas.width, i * PIXEL_SIZE);
            ctx.stroke();
        }

        // Send clear canvas message to server
        if (socket) {
            socket.send(JSON.stringify({
                type: "clear_canvas",
                payload: { userId: myUserId }
            }));
        }
    };

    return (
        <div className="min-h-screen w-full flex items-center justify-center p-4 md:p-8">
            <div className="w-full max-w-7xl">
                {/* Header */}
                <div className="text-center mb-8">
                    <h1 className="text-5xl md:text-6xl font-black text-white mb-2 tracking-tight">
                        🍋 LedLemon
                    </h1>
                    <p className="text-white/80 text-lg">Collaborative Pixel Art Board</p>
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
                    {/* Left Sidebar - Users */}
                    <div className="lg:col-span-3">
                        <div className="bg-white/95 backdrop-blur-md rounded-2xl shadow-2xl p-6 border border-white/20">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-bold text-gray-800 flex items-center gap-2">
                                    <span className="w-3 h-3 bg-green-500 rounded-full animate-pulse"></span>
                                    Online ({users.length})
                                </h3>
                                <button
                                    onClick={() => setShowUsers(!showUsers)}
                                    className="text-gray-500 hover:text-gray-700 transition-colors"
                                >
                                    {showUsers ? "−" : "+"}
                                </button>
                            </div>
                            {showUsers && (
                                <div className="space-y-2 max-h-96 overflow-y-auto">
                                    {users.map((user, idx) => (
                                        <div
                                            key={idx}
                                            className={`px-3 py-2 rounded-lg text-sm font-medium transition-all ${
                                                user === myUserId
                                                    ? "bg-gradient-to-r from-blue-500 to-purple-500 text-white shadow-md"
                                                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                                            }`}
                                        >
                                            {user} {user === myUserId && "👑"}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Center - Canvas */}
                    <div className="lg:col-span-6 flex flex-col items-center gap-6">
                        {/* Color Palette */}
                        <div className="bg-white/95 backdrop-blur-md rounded-2xl shadow-2xl p-6 w-full border border-white/20">
                            <h3 className="text-sm font-bold text-gray-700 mb-4">Color Palette</h3>
                            <div className="flex flex-wrap gap-3 justify-center">
                                {COLORS.map((c) => (
                                    <button
                                        key={c.value}
                                        onClick={() => setColor(c.value)}
                                        className={`w-12 h-12 rounded-xl transition-all transform hover:scale-110 ${
                                            color === c.value
                                                ? `ring-4 ${c.ring} scale-110 shadow-lg`
                                                : "hover:shadow-md"
                                        }`}
                                        style={{ backgroundColor: c.value }}
                                        title={c.name}
                                    />
                                ))}
                            </div>
                            <div className="mt-4 flex gap-2">
                                <button
                                    onClick={handleReset}
                                    className="flex-1 px-4 py-3 bg-gradient-to-r from-red-500 to-pink-500 hover:from-red-600 hover:to-pink-600 text-white font-bold rounded-xl shadow-lg transition-all transform hover:scale-105"
                                >
                                    🗑️ Clear Canvas
                                </button>
                            </div>
                        </div>

                        {/* Canvas */}
                        <div className="bg-white/95 backdrop-blur-md rounded-2xl shadow-2xl p-6 border border-white/20">
                            <canvas
                                ref={canvasRef}
                                width={GRID_SIZE * PIXEL_SIZE}
                                height={GRID_SIZE * PIXEL_SIZE}
                                onMouseDown={handleMouseDown}
                                onMouseMove={handleMouseMove}
                                onMouseUp={handleMouseUp}
                                onMouseLeave={handleMouseLeave}
                                className="cursor-crosshair rounded-lg shadow-inner"
                            />
                        </div>
                    </div>

                    {/* Right Sidebar - Chat */}
                    <div className="lg:col-span-3">
                        <div className="bg-white/95 backdrop-blur-md rounded-2xl shadow-2xl border border-white/20 flex flex-col" style={{ height: 'fit-content' }}>
                            <div className="flex items-center justify-between p-6 pb-4">
                                <h3 className="text-lg font-bold text-gray-800 flex items-center gap-2">
                                    💬 Chat
                                </h3>
                                <button
                                    onClick={() => setShowChat(!showChat)}
                                    className="text-gray-500 hover:text-gray-700 transition-colors"
                                >
                                    {showChat ? "−" : "+"}
                                </button>
                            </div>
                            {showChat && (
                                <div className="px-6 pb-6">
                                    <div className="overflow-y-auto space-y-3 bg-gradient-to-b from-gray-50 to-gray-100 rounded-xl p-4 h-[400px] border-2 border-gray-200">
                                        {chatMessages.length === 0 ? (
                                            <p className="text-gray-400 text-sm text-center mt-8">
                                                No messages yet. Start chatting!
                                            </p>
                                        ) : (
                                            chatMessages.map((msg, idx) => (
                                                <div
                                                    key={idx}
                                                    className={`flex flex-col ${
                                                        msg.isSystem ? "items-center" : msg.userId === myUserId ? "items-end" : "items-start"
                                                    }`}
                                                >
                                                    {!msg.isSystem && (
                                                        <span className="text-xs text-gray-500 mb-1 px-2 font-medium">
                                                            {msg.userId}
                                                        </span>
                                                    )}
                                                    <div
                                                        className={`px-4 py-2.5 rounded-2xl max-w-[85%] break-words shadow-sm ${
                                                            msg.isSystem
                                                                ? "bg-gradient-to-r from-yellow-100 to-orange-100 text-orange-800 text-center italic border border-orange-200"
                                                                : msg.userId === myUserId
                                                                ? "bg-gradient-to-r from-blue-500 to-purple-500 text-white"
                                                                : "bg-white text-gray-800 border border-gray-200"
                                                        }`}
                                                    >
                                                        <p className="text-sm leading-relaxed">{msg.message}</p>
                                                    </div>
                                                </div>
                                            ))
                                        )}
                                        <div ref={chatEndRef} />
                                    </div>
                                    
                                    {/* Divider */}
                                    <div className="border-t-2 border-gray-200 my-4"></div>
                                    
                                    <div className="space-y-2">
                                        <div className="flex gap-2">
                                            <input
                                                type="text"
                                                value={chatInput}
                                                onChange={(e) => setChatInput(e.target.value)}
                                                onKeyPress={handleChatKeyPress}
                                                placeholder="Type a message..."
                                                className="flex-1 px-4 py-3 text-sm border-2 border-gray-300 rounded-xl focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-200 transition-all bg-white"
                                            />
                                            <button
                                                onClick={sendChatMessage}
                                                className="px-6 py-3 bg-gradient-to-r from-blue-500 to-purple-500 hover:from-blue-600 hover:to-purple-600 text-white font-bold rounded-xl shadow-lg transition-all transform hover:scale-105"
                                            >
                                                ➤
                                            </button>
                                        </div>
                                        <button
                                            onClick={clearChat}
                                            className="w-full px-4 py-2.5 bg-gradient-to-r from-orange-500 to-red-500 hover:from-orange-600 hover:to-red-600 text-white text-sm font-bold rounded-xl shadow-lg transition-all transform hover:scale-105"
                                        >
                                            💣 채팅 초토화
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
