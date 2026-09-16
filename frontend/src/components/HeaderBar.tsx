import HeaderMenu from "./HeaderMenu";
import headerIcon from "../assets/images/header_icon.png";

export default function HeaderBar() {
    return (
        <header className="shrink-0 w-full bg-purple-50/80 border-b border-purple-100/80 shadow-xs px-4 sm:px-6 py-2.5 flex items-center justify-between z-10">
            <div className="flex items-center gap-2.5">
                <img
                    src={headerIcon}
                    alt="Obox App"
                    className="h-7 sm:h-8 w-auto object-contain"
                />
                <span className="text-lg font-bold tracking-tight select-none flex items-center gap-1">
                    <span className="text-odoo">Obox</span>
                    <span className="text-amber-500">App</span>
                </span>
            </div>

            <div className="flex items-center gap-2">
                <HeaderMenu />
            </div>
        </header>
    );
}
