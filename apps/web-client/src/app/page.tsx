import PixelCanvas from "./components/PixelCanvas";

export default function Home() {
  return (
    <main 
      className="flex min-h-screen flex-col items-center justify-between p-24"
      style={{
        backgroundImage: "url('https://images.unsplash.com/photo-1505142468610-359e7d316be0?q=80&w=2000')",
        backgroundSize: "cover",
        backgroundPosition: "center",
        backgroundAttachment: "fixed"
      }}
    >
      <PixelCanvas />
    </main>
  );
}
